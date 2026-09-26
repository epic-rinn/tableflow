// Package members implements customer (member) accounts: signup, email
// verification, sign-in/out and password reset. Member credentials are kept
// separate from staff credentials and never carry staff authority.
package members

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/epic-rinn/tableflow/src/api/internal/identity"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/mail"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/password"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/throttle"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/token"
)

//go:embed sql/*.sql
var sqlFiles embed.FS

func q(name string) string {
	b, err := sqlFiles.ReadFile("sql/" + name + ".sql")
	if err != nil {
		panic("members: missing SQL " + name)
	}
	return string(b)
}

// Lifetimes and throttle limits (per window).
const (
	SessionAbsolute  = 30 * 24 * time.Hour
	SessionIdle      = 7 * 24 * time.Hour
	verifyLifetime   = 24 * time.Hour
	resetLifetime    = time.Hour
	window           = 10 * time.Minute
	loginPerEmail    = 10
	loginPerIP       = 100
	signupPerIP      = 20
	signupPerEmail   = 5 // bounds "account exists" notices to one mailbox
	resendPerIP      = 30
	resetPerEmail    = 5
	resetPerIP       = 30
	resendPerEmail   = 5
	tokenSubmitPerIP = 30
	purposeVerify    = "verify"
	purposeReset     = "reset"
)

// Errors mapped by the HTTP layer.
var (
	ErrUnauthenticated    = errors.New("member unauthenticated")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrTokenInvalid       = errors.New("token invalid")
)

// ValidationError carries per-field messages.
type ValidationError struct{ Fields map[string]string }

func (e *ValidationError) Error() string { return "validation failed" }

// Member is an authenticated member session.
type Member struct {
	SessionID     string `json:"-"`
	ID            string `json:"member_id"`
	Email         string `json:"email"`
	Locale        string `json:"locale"`
	EmailVerified bool   `json:"email_verified"`
}

// Service implements member use cases.
type Service struct {
	pool      *pgxpool.Pool
	limiter   *throttle.Limiter
	hasher    *password.Hasher
	outbox    mail.Outbox
	publicURL string
}

// NewService builds the service; publicURL is the PWA origin for email links.
func NewService(pool *pgxpool.Pool, hasher *password.Hasher, outbox mail.Outbox, publicURL string) *Service {
	return &Service{pool: pool, limiter: throttle.New(pool), hasher: hasher, outbox: outbox, publicURL: publicURL}
}

func validLocale(l string) bool { return l == "th" || l == "en" }

// issueToken revokes open tokens of the purpose and creates a new one.
func issueToken(ctx context.Context, tx pgx.Tx, memberID, purpose string, lifetime time.Duration) (string, error) {
	if _, err := tx.Exec(ctx, q("revoke_open_tokens"), memberID, purpose); err != nil {
		return "", err
	}
	raw, hash := token.New()
	if _, err := tx.Exec(ctx, q("insert_token"), memberID, purpose, hash, lifetime); err != nil {
		return "", err
	}
	return raw, nil
}

// Signup registers an account and emails a verification link. For an
// existing email it emails a notice instead; the caller sees the same result.
func (s *Service) Signup(ctx context.Context, email, pw, locale string, ip netip.Addr) error {
	fields := map[string]string{}
	norm, ok := identity.NormalizeEmail(email)
	if !ok {
		fields["email"] = "must be a valid email address"
	}
	if msg := password.Validate(pw); msg != "" {
		fields["password"] = msg
	}
	if !validLocale(locale) {
		fields["locale"] = "must be th or en"
	}
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	if err := s.limiter.Hit(ctx, throttle.IPKey("member-signup:ip", ip), signupPerIP, window); err != nil {
		return err
	}
	if err := s.limiter.Hit(ctx, throttle.HashedKey("member-signup:email", norm), signupPerEmail, window); err != nil {
		return err
	}
	hash, err := s.hasher.Hash(ctx, pw)
	if err != nil {
		return err
	}
	var msg mail.Message
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var id string
		err := tx.QueryRow(ctx, q("insert_account"), norm, hash, locale).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			msg = existingAccountMessage(norm, locale, s.publicURL)
			return nil
		}
		if err != nil {
			return err
		}
		raw, err := issueToken(ctx, tx, id, purposeVerify, verifyLifetime)
		if err != nil {
			return err
		}
		msg = verifyMessage(norm, locale, s.publicURL, raw)
		return nil
	})
	if err != nil {
		return err
	}
	s.outbox.Enqueue(msg) // after commit; delivery is asynchronous
	return nil
}

// ResendVerification emails a fresh verification link to an unverified
// account. Always succeeds from the caller's perspective.
func (s *Service) ResendVerification(ctx context.Context, email string, ip netip.Addr) error {
	norm, ok := identity.NormalizeEmail(email)
	if !ok {
		return &ValidationError{Fields: map[string]string{"email": "must be a valid email address"}}
	}
	if err := s.limiter.Hit(ctx, throttle.IPKey("member-resend:ip", ip), resendPerIP, window); err != nil {
		return err
	}
	if err := s.limiter.Hit(ctx, throttle.HashedKey("member-resend:email", norm), resendPerEmail, window); err != nil {
		return err
	}
	var msg *mail.Message
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var id, hash, locale string
		var verified bool
		err := tx.QueryRow(ctx, q("find_account"), norm).Scan(&id, &hash, &locale, &verified)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && verified) {
			return nil
		}
		if err != nil {
			return err
		}
		raw, err := issueToken(ctx, tx, id, purposeVerify, verifyLifetime)
		if err != nil {
			return err
		}
		m := verifyMessage(norm, locale, s.publicURL, raw)
		msg = &m
		return nil
	})
	if err == nil && msg != nil {
		s.outbox.Enqueue(*msg)
	}
	return err
}

// consumeToken locks the account then the token (lock order: account →
// token) and marks the token used, returning the member ID.
func consumeToken(ctx context.Context, tx pgx.Tx, pool *pgxpool.Pool, raw, purpose string) (string, error) {
	hash, ok := token.Hash(raw)
	if !ok {
		return "", ErrTokenInvalid
	}
	var memberID string
	if err := pool.QueryRow(ctx, q("find_token_member"), hash, purpose).Scan(&memberID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrTokenInvalid
		}
		return "", err
	}
	var email string
	if err := tx.QueryRow(ctx, q("lock_account"), memberID).Scan(&email); err != nil {
		return "", err
	}
	var tokenID string
	err := tx.QueryRow(ctx, q("lock_usable_token"), hash, memberID, purpose).Scan(&tokenID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrTokenInvalid
	}
	if err != nil {
		return "", err
	}
	tag, err := tx.Exec(ctx, q("use_token"), tokenID)
	if err != nil {
		return "", err
	}
	if tag.RowsAffected() != 1 {
		return "", ErrTokenInvalid
	}
	return memberID, nil
}

// Verify confirms an email address with a single-use token.
func (s *Service) Verify(ctx context.Context, raw string, ip netip.Addr) error {
	if err := s.limiter.Hit(ctx, throttle.IPKey("member-token:ip", ip), tokenSubmitPerIP, window); err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		id, err := consumeToken(ctx, tx, s.pool, raw, purposeVerify)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, q("mark_verified"), id)
		return err
	})
}

// RequestReset emails a reset link when the account exists. Always succeeds
// from the caller's perspective.
func (s *Service) RequestReset(ctx context.Context, email string, ip netip.Addr) error {
	norm, ok := identity.NormalizeEmail(email)
	if !ok {
		return &ValidationError{Fields: map[string]string{"email": "must be a valid email address"}}
	}
	if err := s.limiter.Hit(ctx, throttle.IPKey("member-reset:ip", ip), resetPerIP, window); err != nil {
		return err
	}
	if err := s.limiter.Hit(ctx, throttle.HashedKey("member-reset:email", norm), resetPerEmail, window); err != nil {
		return err
	}
	var msg *mail.Message
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var id, hash, locale string
		var verified bool
		err := tx.QueryRow(ctx, q("find_account"), norm).Scan(&id, &hash, &locale, &verified)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		raw, err := issueToken(ctx, tx, id, purposeReset, resetLifetime)
		if err != nil {
			return err
		}
		m := resetMessage(norm, locale, s.publicURL, raw)
		msg = &m
		return nil
	})
	if err == nil && msg != nil {
		s.outbox.Enqueue(*msg)
	}
	return err
}

// ConfirmReset sets a new password, verifies the email and revokes every
// session of the account.
func (s *Service) ConfirmReset(ctx context.Context, raw, pw string, ip netip.Addr) error {
	if msg := password.Validate(pw); msg != "" {
		return &ValidationError{Fields: map[string]string{"new_password": msg}}
	}
	if err := s.limiter.Hit(ctx, throttle.IPKey("member-token:ip", ip), tokenSubmitPerIP, window); err != nil {
		return err
	}
	hash, err := s.hasher.Hash(ctx, pw) // outside the transaction
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		id, err := consumeToken(ctx, tx, s.pool, raw, purposeReset)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, q("reset_password"), id, hash); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, q("revoke_member_sessions"), id)
		return err
	})
}

// Login verifies credentials and creates a session; the raw token is
// returned once for the cookie.
func (s *Service) Login(ctx context.Context, email, pw string, ip netip.Addr) (Member, string, time.Time, error) {
	norm, ok := identity.NormalizeEmail(email)
	if !ok || pw == "" || len(pw) > password.MaxBytes {
		return Member{}, "", time.Time{}, &ValidationError{Fields: map[string]string{"email": "enter an email and password"}}
	}
	emailBucket := throttle.HashedKey("member-login:email", norm)
	if err := s.limiter.Hit(ctx, throttle.IPKey("member-login:ip", ip), loginPerIP, window); err != nil {
		return Member{}, "", time.Time{}, err
	}
	if err := s.limiter.Hit(ctx, emailBucket, loginPerEmail, window); err != nil {
		return Member{}, "", time.Time{}, err
	}
	var id, hash, locale string
	var verified bool
	err := s.pool.QueryRow(ctx, q("find_account"), norm).Scan(&id, &hash, &locale, &verified)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Member{}, "", time.Time{}, fmt.Errorf("find member: %w", err)
	}
	match, err := s.hasher.Verify(ctx, pw, hash) // dummy hash when unknown
	if err != nil {
		return Member{}, "", time.Time{}, err
	}
	if !match {
		return Member{}, "", time.Time{}, ErrInvalidCredentials
	}
	raw, tokenHash := token.New()
	var sessionID string
	var expires time.Time
	if err := s.pool.QueryRow(ctx, q("insert_session"), id, tokenHash, SessionAbsolute).Scan(&sessionID, &expires); err != nil {
		return Member{}, "", time.Time{}, fmt.Errorf("create member session: %w", err)
	}
	if err := s.limiter.Clear(ctx, emailBucket); err != nil {
		return Member{}, "", time.Time{}, err
	}
	return Member{SessionID: sessionID, ID: id, Email: norm, Locale: locale, EmailVerified: verified}, raw, expires, nil
}

// Authenticate resolves a member session cookie.
func (s *Service) Authenticate(ctx context.Context, raw string) (Member, error) {
	hash, ok := token.Hash(raw)
	if !ok {
		return Member{}, ErrUnauthenticated
	}
	var m Member
	var stale bool
	err := s.pool.QueryRow(ctx, q("authenticate_session"), hash, SessionIdle).
		Scan(&m.SessionID, &m.ID, &m.Email, &m.Locale, &m.EmailVerified, &stale)
	if errors.Is(err, pgx.ErrNoRows) {
		return Member{}, ErrUnauthenticated
	}
	if err != nil {
		return Member{}, fmt.Errorf("authenticate member: %w", err)
	}
	if stale {
		if _, err := s.pool.Exec(ctx, q("touch_session"), m.SessionID); err != nil {
			return Member{}, err
		}
	}
	return m, nil
}

// Logout revokes the current session.
func (s *Service) Logout(ctx context.Context, m Member) error {
	_, err := s.pool.Exec(ctx, q("revoke_session"), m.SessionID)
	return err
}

// Purge deletes long-expired sessions and tokens.
func (s *Service) Purge(ctx context.Context) (int64, error) {
	var n int64
	err := s.pool.QueryRow(ctx, q("purge")).Scan(&n)
	return n, err
}
