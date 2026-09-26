// Package identity implements staff accounts, sessions and authorization.
package identity

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/epic-rinn/tableflow/src/api/internal/platform/throttle"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/token"
)

//go:embed sql/*.sql
var sqlFiles embed.FS

// q returns an embedded statement; a missing name is a programming error.
func q(name string) string {
	b, err := sqlFiles.ReadFile("sql/" + name + ".sql")
	if err != nil {
		panic("identity: missing SQL " + name)
	}
	return string(b)
}

// Roles, in canonical order.
const (
	RoleCashier = "cashier"
	RoleHost    = "host"
	RoleKitchen = "kitchen"
	RoleManager = "manager"
)

var allRoles = []string{RoleCashier, RoleHost, RoleKitchen, RoleManager}

// ActivationLifetime bounds how long an invitation link is usable.
const ActivationLifetime = 72 * time.Hour

// Throttle limits (fixed windows).
const (
	throttleWindow     = 10 * time.Minute
	loginPerEmail      = 10
	loginPerIP         = 100 // shared by everyone behind one NAT/proxy address
	activationPerIP    = 20
	staffPageDefault   = 25
	staffPageMax       = 100
	maxReasonRunes     = 500
	maxDisplayNameRune = 100
)

// Service errors mapped to HTTP responses by the handlers.
var (
	ErrUnauthenticated    = errors.New("unauthenticated")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrForbidden          = errors.New("forbidden")
	ErrNotFound           = errors.New("not found")
	ErrVersionConflict    = errors.New("version conflict")
	ErrEmailTaken         = errors.New("email taken")
	ErrLastManager        = errors.New("last manager")
	ErrTokenInvalid       = errors.New("token invalid")
	ErrNotInvited         = errors.New("not invited")
	ErrAccountDisabled    = errors.New("account disabled")
	ErrInvalidCursor      = errors.New("invalid cursor")
	ErrBootstrapDone      = errors.New("a branch already exists")
)

// ValidationError carries per-field messages.
type ValidationError struct{ Fields map[string]string }

func (e *ValidationError) Error() string { return "validation failed" }

// Principal is an authenticated staff session.
type Principal struct {
	SessionID   string
	StaffID     string
	BranchID    string
	Email       string
	DisplayName string
	Roles       []string
}

// Has reports whether the principal holds role.
func (p Principal) Has(role string) bool { return slices.Contains(p.Roles, role) }

// Staff is the manager-visible account view.
type Staff struct {
	ID          string    `json:"id"`
	BranchID    string    `json:"branch_id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Status      string    `json:"status"`
	Roles       []string  `json:"roles"`
	Version     int       `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
}

// Activation is a one-time token shown to the issuing manager.
type Activation struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Service owns identity use cases and their transactions.
type Service struct {
	pool     *pgxpool.Pool
	limiter  *throttle.Limiter
	hasher   *Hasher
	idle     time.Duration
	absolute time.Duration
}

// NewService builds the identity service.
func NewService(pool *pgxpool.Pool, hasher *Hasher, idle, absolute time.Duration) *Service {
	return &Service{pool: pool, limiter: throttle.New(pool), hasher: hasher, idle: idle, absolute: absolute}
}

// SessionLifetime is the absolute session lifetime (for cookie expiry).
func (s *Service) SessionLifetime() time.Duration { return s.absolute }

// NormalizeEmail trims and lower-cases an address and checks its shape.
func NormalizeEmail(raw string) (string, bool) {
	e := strings.ToLower(strings.TrimSpace(raw))
	if len(e) < 3 || len(e) > 254 {
		return "", false
	}
	addr, err := mail.ParseAddress(e)
	if err != nil || addr.Address != e || addr.Name != "" {
		return "", false
	}
	return e, true
}

func normalizeRoles(in []string) ([]string, bool) {
	if len(in) == 0 || len(in) > len(allRoles) {
		return nil, false
	}
	out := make([]string, 0, len(in))
	for _, r := range in {
		if !slices.Contains(allRoles, r) {
			return nil, false
		}
		if !slices.Contains(out, r) {
			out = append(out, r)
		}
	}
	slices.Sort(out)
	return out, true
}

func validDisplayName(n string) (string, bool) {
	n = strings.TrimSpace(n)
	count := len([]rune(n))
	return n, count >= 1 && count <= maxDisplayNameRune
}

// Purge deletes expired throttle buckets and long-expired sessions.
func (s *Service) Purge(ctx context.Context) (int64, error) {
	n, err := s.limiter.Purge(ctx)
	if err != nil {
		return 0, fmt.Errorf("throttle purge: %w", err)
	}
	tag, err := s.pool.Exec(ctx, q("session_purge"))
	if err != nil {
		return n, fmt.Errorf("session purge: %w", err)
	}
	return n + tag.RowsAffected(), nil
}

// Authenticate resolves a raw session cookie value.
func (s *Service) Authenticate(ctx context.Context, raw string) (Principal, error) {
	hash, ok := token.Hash(raw)
	if !ok {
		return Principal{}, ErrUnauthenticated
	}
	var p Principal
	var stale bool
	err := s.pool.QueryRow(ctx, q("authenticate_session"), hash, s.idle).
		Scan(&p.SessionID, &p.StaffID, &p.BranchID, &p.Email, &p.DisplayName, &p.Roles, &stale)
	if errors.Is(err, pgx.ErrNoRows) {
		return Principal{}, ErrUnauthenticated
	}
	if err != nil {
		return Principal{}, fmt.Errorf("authenticate: %w", err)
	}
	if stale {
		if _, err := s.pool.Exec(ctx, q("touch_session"), p.SessionID); err != nil {
			return Principal{}, fmt.Errorf("touch session: %w", err)
		}
	}
	return p, nil
}

// Login verifies credentials and creates a session. The raw token is
// returned once for the cookie.
func (s *Service) Login(ctx context.Context, email, password string, ip netip.Addr) (Principal, string, time.Time, error) {
	norm, ok := NormalizeEmail(email)
	if !ok || password == "" || len(password) > maxPasswordBytes {
		return Principal{}, "", time.Time{}, &ValidationError{Fields: map[string]string{"email": "enter an email and password"}}
	}
	emailBucket := throttle.HashedKey("login:email", norm)
	if err := s.limiter.Hit(ctx, throttle.IPKey("login:ip", ip), loginPerIP, throttleWindow); err != nil {
		return Principal{}, "", time.Time{}, err
	}
	if err := s.limiter.Hit(ctx, emailBucket, loginPerEmail, throttleWindow); err != nil {
		return Principal{}, "", time.Time{}, err
	}

	var id, status string
	var hash *string
	err := s.pool.QueryRow(ctx, q("find_login_account"), norm).Scan(&id, &hash, &status)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Principal{}, "", time.Time{}, fmt.Errorf("find account: %w", err)
	}
	encoded := ""
	if hash != nil {
		encoded = *hash
	}
	match, err := s.hasher.Verify(ctx, password, encoded)
	if err != nil {
		return Principal{}, "", time.Time{}, fmt.Errorf("verify password: %w", err)
	}
	if !match || status != "active" {
		return Principal{}, "", time.Time{}, ErrInvalidCredentials
	}

	raw, tokenHash := token.New()
	var sessionID string
	var expires time.Time
	if err := s.pool.QueryRow(ctx, q("insert_session"), id, tokenHash, s.absolute).Scan(&sessionID, &expires); err != nil {
		return Principal{}, "", time.Time{}, fmt.Errorf("create session: %w", err)
	}
	if err := s.limiter.Clear(ctx, emailBucket); err != nil {
		return Principal{}, "", time.Time{}, fmt.Errorf("clear throttle: %w", err)
	}
	p, err := s.Authenticate(ctx, raw)
	if err != nil {
		return Principal{}, "", time.Time{}, err
	}
	return p, raw, expires, nil
}

// Logout revokes the principal's current session.
func (s *Service) Logout(ctx context.Context, p Principal) error {
	_, err := s.pool.Exec(ctx, q("revoke_session"), p.SessionID)
	return err
}

// Activate sets the password of an invited account using a single-use token.
func (s *Service) Activate(ctx context.Context, rawToken, password, displayName string, ip netip.Addr, requestID string) error {
	if err := s.limiter.Hit(ctx, throttle.IPKey("activate:ip", ip), activationPerIP, throttleWindow); err != nil {
		return err
	}
	fields := map[string]string{}
	if msg := validatePassword(password); msg != "" {
		fields["password"] = msg
	}
	name, nameOK := validDisplayName(displayName)
	if displayName != "" && !nameOK {
		fields["display_name"] = "must be 1–100 characters"
	}
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	tokenHash, ok := token.Hash(rawToken)
	if !ok {
		return ErrTokenInvalid
	}
	var accountID string
	if err := s.pool.QueryRow(ctx, q("find_activation_account"), tokenHash).Scan(&accountID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrTokenInvalid
		}
		return fmt.Errorf("find token: %w", err)
	}
	// Hash before the transaction: no locks held during slow work.
	pwHash, err := s.hasher.Hash(ctx, password)
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var branchID, email, currentName string
		err := tx.QueryRow(ctx, q("lock_invited_account"), accountID).Scan(&branchID, &email, &currentName)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrTokenInvalid
		}
		if err != nil {
			return err
		}
		var tokenID string
		err = tx.QueryRow(ctx, q("lock_usable_token"), tokenHash, accountID).Scan(&tokenID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrTokenInvalid
		}
		if err != nil {
			return err
		}
		if !nameOK {
			name = currentName
		}
		if err := execOne(ctx, tx, q("activate_account"), accountID, pwHash, name); err != nil {
			return err
		}
		if err := execOne(ctx, tx, q("use_token"), tokenID); err != nil {
			return err
		}
		return audit(ctx, tx, branchID, &accountID, "staff.activated", accountID, nil, requestID, nil)
	})
}

// execOne executes a statement that must affect exactly one row.
func execOne(ctx context.Context, tx pgx.Tx, sql string, args ...any) error {
	tag, err := tx.Exec(ctx, sql, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("expected 1 row, affected %d", tag.RowsAffected())
	}
	return nil
}

func audit(ctx context.Context, tx pgx.Tx, branchID string, actor *string, action, resourceID string, reason *string, requestID string, details map[string]any) error {
	if details == nil {
		details = map[string]any{}
	}
	b, err := json.Marshal(details)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, q("insert_audit"), branchID, actor, action, "staff_account", resourceID, reason, requestID, b)
	return err
}

// adminTx runs a staff-administration mutation for branchID. Lock order:
// branch row → acting session/account (FOR SHARE) → target account.
func (s *Service) adminTx(ctx context.Context, p Principal, branchID string, fn func(pgx.Tx, Principal) error) error {
	if branchID != p.BranchID {
		return ErrNotFound
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var locked string
		if err := tx.QueryRow(ctx, q("lock_branch"), branchID).Scan(&locked); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		actor, err := s.revalidate(ctx, tx, p)
		if err != nil {
			return err
		}
		if actor.BranchID != branchID {
			return ErrNotFound
		}
		if !actor.Has(RoleManager) {
			return ErrForbidden
		}
		return fn(tx, actor)
	})
}

// revalidate re-reads the acting session under FOR SHARE in tx.
func (s *Service) revalidate(ctx context.Context, tx pgx.Tx, p Principal) (Principal, error) {
	actor := Principal{SessionID: p.SessionID}
	err := tx.QueryRow(ctx, q("revalidate_actor"), p.SessionID, s.idle).
		Scan(&actor.StaffID, &actor.BranchID, &actor.Email, &actor.DisplayName, &actor.Roles)
	if errors.Is(err, pgx.ErrNoRows) {
		return Principal{}, ErrUnauthenticated
	}
	return actor, err
}

func (s *Service) getStaff(ctx context.Context, db interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, id string) (Staff, error) {
	var st Staff
	err := db.QueryRow(ctx, q("get_staff"), id).
		Scan(&st.ID, &st.BranchID, &st.Email, &st.DisplayName, &st.Status, &st.Version, &st.CreatedAt, &st.Roles)
	return st, err
}

func (s *Service) issueToken(ctx context.Context, tx pgx.Tx, accountID string, issuer *string) (Activation, error) {
	raw, hash := token.New()
	a := Activation{Token: raw}
	err := tx.QueryRow(ctx, q("insert_token"), accountID, hash, ActivationLifetime, issuer).Scan(&a.ExpiresAt)
	return a, err
}

// Invite creates an invited account with roles and a one-time activation token.
func (s *Service) Invite(ctx context.Context, p Principal, branchID, email, displayName string, roles []string, requestID string) (Staff, Activation, error) {
	fields := map[string]string{}
	norm, ok := NormalizeEmail(email)
	if !ok {
		fields["email"] = "must be a valid email address"
	}
	name, ok := validDisplayName(displayName)
	if !ok {
		fields["display_name"] = "must be 1–100 characters"
	}
	rs, ok := normalizeRoles(roles)
	if !ok {
		fields["roles"] = "choose at least one of cashier, host, kitchen, manager"
	}
	if len(fields) > 0 {
		return Staff{}, Activation{}, &ValidationError{Fields: fields}
	}
	var st Staff
	var act Activation
	err := s.adminTx(ctx, p, branchID, func(tx pgx.Tx, actor Principal) error {
		var id string
		err := tx.QueryRow(ctx, q("insert_account"), branchID, norm, name).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrEmailTaken
		}
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, q("insert_roles"), id, rs); err != nil {
			return err
		}
		if act, err = s.issueToken(ctx, tx, id, &actor.StaffID); err != nil {
			return err
		}
		if err := audit(ctx, tx, branchID, &actor.StaffID, "staff.invited", id, nil, requestID, map[string]any{"roles": rs}); err != nil {
			return err
		}
		st, err = s.getStaff(ctx, tx, id)
		return err
	})
	return st, act, err
}

// lockTarget locks a staff account in the actor's branch and checks version.
func lockTarget(ctx context.Context, tx pgx.Tx, id, branchID string, expectedVersion *int) (string, error) {
	var status string
	var version int
	err := tx.QueryRow(ctx, q("lock_target"), id, branchID).Scan(&status, &version)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if expectedVersion != nil && *expectedVersion != version {
		return "", ErrVersionConflict
	}
	return status, nil
}

// ReissueActivation replaces an invited account's activation token.
func (s *Service) ReissueActivation(ctx context.Context, p Principal, staffID, requestID string) (Activation, error) {
	var act Activation
	err := s.adminTx(ctx, p, p.BranchID, func(tx pgx.Tx, actor Principal) error {
		status, err := lockTarget(ctx, tx, staffID, actor.BranchID, nil)
		if err != nil {
			return err
		}
		if status != "invited" {
			return ErrNotInvited
		}
		if _, err := tx.Exec(ctx, q("revoke_open_tokens"), staffID); err != nil {
			return err
		}
		if act, err = s.issueToken(ctx, tx, staffID, &actor.StaffID); err != nil {
			return err
		}
		return audit(ctx, tx, actor.BranchID, &actor.StaffID, "staff.activation_reissued", staffID, nil, requestID, nil)
	})
	return act, err
}

func requireManagerRemains(ctx context.Context, tx pgx.Tx, branchID string) error {
	var n int
	if err := tx.QueryRow(ctx, q("count_active_managers"), branchID).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		return ErrLastManager
	}
	return nil
}

// SetRoles replaces a staff member's roles and revokes their sessions.
func (s *Service) SetRoles(ctx context.Context, p Principal, staffID string, expectedVersion int, roles []string, requestID string) (Staff, error) {
	rs, ok := normalizeRoles(roles)
	if !ok {
		return Staff{}, &ValidationError{Fields: map[string]string{"roles": "choose at least one of cashier, host, kitchen, manager"}}
	}
	var st Staff
	err := s.adminTx(ctx, p, p.BranchID, func(tx pgx.Tx, actor Principal) error {
		status, err := lockTarget(ctx, tx, staffID, actor.BranchID, &expectedVersion)
		if err != nil {
			return err
		}
		if status == "disabled" {
			return ErrAccountDisabled
		}
		for _, stmt := range []struct {
			sql  string
			args []any
		}{
			{q("delete_roles"), []any{staffID}},
			{q("insert_roles"), []any{staffID, rs}},
			{q("revoke_account_sessions"), []any{staffID}},
		} {
			if _, err := tx.Exec(ctx, stmt.sql, stmt.args...); err != nil {
				return err
			}
		}
		if err := execOne(ctx, tx, q("bump_account"), staffID); err != nil {
			return err
		}
		if err := requireManagerRemains(ctx, tx, actor.BranchID); err != nil {
			return err
		}
		if err := audit(ctx, tx, actor.BranchID, &actor.StaffID, "staff.roles_changed", staffID, nil, requestID, map[string]any{"roles": rs}); err != nil {
			return err
		}
		st, err = s.getStaff(ctx, tx, staffID)
		return err
	})
	return st, err
}

// Deactivate disables an account, revoking its sessions and open tokens.
func (s *Service) Deactivate(ctx context.Context, p Principal, staffID string, expectedVersion int, reason, requestID string) (Staff, error) {
	reason = strings.TrimSpace(reason)
	if n := len([]rune(reason)); n < 1 || n > maxReasonRunes {
		return Staff{}, &ValidationError{Fields: map[string]string{"reason": "must be 1–500 characters"}}
	}
	var st Staff
	err := s.adminTx(ctx, p, p.BranchID, func(tx pgx.Tx, actor Principal) error {
		status, err := lockTarget(ctx, tx, staffID, actor.BranchID, &expectedVersion)
		if err != nil {
			return err
		}
		if status == "disabled" {
			return ErrAccountDisabled
		}
		if err := execOne(ctx, tx, q("disable_account"), staffID); err != nil {
			return err
		}
		for _, name := range []string{"revoke_account_sessions", "revoke_open_tokens"} {
			if _, err := tx.Exec(ctx, q(name), staffID); err != nil {
				return err
			}
		}
		if err := requireManagerRemains(ctx, tx, actor.BranchID); err != nil {
			return err
		}
		if err := audit(ctx, tx, actor.BranchID, &actor.StaffID, "staff.deactivated", staffID, &reason, requestID, nil); err != nil {
			return err
		}
		st, err = s.getStaff(ctx, tx, staffID)
		return err
	})
	return st, err
}

type cursor struct {
	Branch    string    `json:"b"`
	CreatedAt time.Time `json:"t"`
	ID        string    `json:"i"`
}

// ListStaff returns one keyset page of a branch's staff (managers only).
func (s *Service) ListStaff(ctx context.Context, p Principal, branchID, after string, limit int) ([]Staff, string, error) {
	if branchID != p.BranchID {
		return nil, "", ErrNotFound
	}
	if !p.Has(RoleManager) {
		return nil, "", ErrForbidden
	}
	if limit <= 0 {
		limit = staffPageDefault
	}
	limit = min(limit, staffPageMax)
	c := cursor{CreatedAt: time.Unix(0, 0).UTC().AddDate(-100, 0, 0), ID: "00000000-0000-0000-0000-000000000000"}
	if after != "" {
		var err error
		if c, err = decodeCursor(after); err != nil || c.Branch != branchID {
			return nil, "", ErrInvalidCursor
		}
	}
	rows, err := s.pool.Query(ctx, q("list_staff"), branchID, c.CreatedAt, c.ID, limit+1)
	if err != nil {
		return nil, "", err
	}
	items, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Staff, error) {
		var st Staff
		err := r.Scan(&st.ID, &st.BranchID, &st.Email, &st.DisplayName, &st.Status, &st.Version, &st.CreatedAt, &st.Roles)
		return st, err
	})
	if err != nil {
		return nil, "", err
	}
	next := ""
	if len(items) > limit {
		items = items[:limit]
		last := items[len(items)-1]
		next = encodeCursor(cursor{Branch: branchID, CreatedAt: last.CreatedAt, ID: last.ID})
	}
	return items, next, nil
}

// Bootstrap creates the first branch and an invited manager. It refuses once
// any branch exists, so it cannot be used to add managers later.
func (s *Service) Bootstrap(ctx context.Context, branchName, email, displayName string) (string, Staff, Activation, error) {
	fields := map[string]string{}
	norm, ok := NormalizeEmail(email)
	if !ok {
		fields["email"] = "must be a valid email address"
	}
	name, ok := validDisplayName(displayName)
	if !ok {
		fields["display_name"] = "must be 1–100 characters"
	}
	branchName = strings.TrimSpace(branchName)
	if n := len([]rune(branchName)); n < 1 || n > 100 {
		fields["branch_name"] = "must be 1–100 characters"
	}
	if len(fields) > 0 {
		return "", Staff{}, Activation{}, &ValidationError{Fields: fields}
	}
	var branchID string
	var st Staff
	var act Activation
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, q("lock_bootstrap")); err != nil {
			return err
		}
		var exists bool
		if err := tx.QueryRow(ctx, q("any_branch")).Scan(&exists); err != nil {
			return err
		}
		if exists {
			return ErrBootstrapDone
		}
		if err := tx.QueryRow(ctx, q("insert_branch"), branchName).Scan(&branchID); err != nil {
			return err
		}
		var id string
		if err := tx.QueryRow(ctx, q("insert_account"), branchID, norm, name).Scan(&id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, q("insert_roles"), id, []string{RoleManager}); err != nil {
			return err
		}
		var err error
		if act, err = s.issueToken(ctx, tx, id, nil); err != nil {
			return err
		}
		if err := audit(ctx, tx, branchID, nil, "staff.bootstrapped", id, nil, "bootstrap", nil); err != nil {
			return err
		}
		st, err = s.getStaff(ctx, tx, id)
		return err
	})
	return branchID, st, act, err
}
