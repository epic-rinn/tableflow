// Package access implements guest capabilities (queue/visit QR tokens),
// capability-derived guest sessions and anonymous bootstrap sessions.
//
// Domain modules issue, rotate and revoke capabilities inside their own
// transactions, after locking the owning ticket or visit.
package access

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/epic-rinn/tableflow/src/api/internal/platform/throttle"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/token"
)

//go:embed sql/*.sql
var sqlFiles embed.FS

func q(name string) string {
	b, err := sqlFiles.ReadFile("sql/" + name + ".sql")
	if err != nil {
		panic("access: missing SQL " + name)
	}
	return string(b)
}

// Capability kinds.
const (
	KindQueue = "queue"
	KindVisit = "visit"
)

// Lifetimes and limits.
const (
	GuestSessionLifetime     = 12 * time.Hour
	AnonymousSessionLifetime = 24 * time.Hour
	throttleWindow           = 10 * time.Minute
	exchangePerIP            = 120
	anonymousPerIP           = 60
)

// Errors mapped to HTTP responses.
var (
	ErrTokenInvalid    = errors.New("capability token invalid")
	ErrUnauthenticated = errors.New("guest unauthenticated")
	ErrNoCapability    = errors.New("no active capability for resource")
)

// Guest is a capability-derived session.
type Guest struct {
	SessionID    string    `json:"-"`
	CapabilityID string    `json:"-"`
	BranchID     string    `json:"branch_id"`
	Kind         string    `json:"kind"`
	ResourceID   string    `json:"resource_id"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// Anonymous is a bootstrap browser identity.
type Anonymous struct {
	ID        string
	ExpiresAt time.Time
}

// Service implements access use cases.
type Service struct {
	pool    *pgxpool.Pool
	limiter *throttle.Limiter
}

// NewService builds the access service.
func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, limiter: throttle.New(pool)}
}

func validKind(k string) bool { return k == KindQueue || k == KindVisit }

// IssueCapability creates the capability for a resource and returns its raw
// token (for the QR code / link). One capability exists per resource.
func IssueCapability(ctx context.Context, tx pgx.Tx, branchID, kind, resourceID string, expiresAt *time.Time) (string, error) {
	if !validKind(kind) {
		return "", fmt.Errorf("access: invalid kind %q", kind)
	}
	raw, hash := token.New()
	var id string
	if err := tx.QueryRow(ctx, q("capability_issue"), branchID, kind, resourceID, hash, expiresAt).Scan(&id); err != nil {
		return "", fmt.Errorf("issue capability: %w", err)
	}
	return raw, nil
}

// RotateCapability replaces the token and generation. Every guest session
// derived from earlier generations stops working immediately.
func RotateCapability(ctx context.Context, tx pgx.Tx, kind, resourceID string) (string, int, error) {
	raw, hash := token.New()
	var id string
	var generation int
	err := tx.QueryRow(ctx, q("capability_rotate"), kind, resourceID, hash).Scan(&id, &generation)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", 0, ErrNoCapability
	}
	if err != nil {
		return "", 0, fmt.Errorf("rotate capability: %w", err)
	}
	if _, err := tx.Exec(ctx, q("guest_revoke_capability"), id); err != nil {
		return "", 0, fmt.Errorf("revoke guest sessions: %w", err)
	}
	return raw, generation, nil
}

// RevokeCapability ends a capability and its guest sessions (terminal
// ticket/visit states).
func RevokeCapability(ctx context.Context, tx pgx.Tx, kind, resourceID string) error {
	var id string
	err := tx.QueryRow(ctx, q("capability_revoke"), kind, resourceID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNoCapability
	}
	if err != nil {
		return fmt.Errorf("revoke capability: %w", err)
	}
	if _, err := tx.Exec(ctx, q("guest_revoke_capability"), id); err != nil {
		return fmt.Errorf("revoke guest sessions: %w", err)
	}
	return nil
}

// ResolveCapability maps a raw capability token to its branch and resource
// without creating a session; staff use it to find a visit's bill from the
// diner's QR (BIL-001). Revoked or expired tokens do not resolve.
func ResolveCapability(ctx context.Context, db interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, kind, rawToken string) (branchID, resourceID string, err error) {
	hash, ok := token.Hash(rawToken)
	if !ok || !validKind(kind) {
		return "", "", ErrTokenInvalid
	}
	var id string
	var generation int
	var expires *time.Time
	err = db.QueryRow(ctx, q("capability_lookup"), hash, kind).Scan(&id, &branchID, &resourceID, &generation, &expires)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", ErrTokenInvalid
	}
	if err != nil {
		return "", "", fmt.Errorf("lookup capability: %w", err)
	}
	return branchID, resourceID, nil
}

// Exchange turns a QR token into a new guest session. The capability is not
// consumed, so every diner at the table can exchange the same token.
func (s *Service) Exchange(ctx context.Context, rawToken, kind string, ip netip.Addr) (Guest, string, error) {
	if err := s.limiter.Hit(ctx, throttle.IPKey("capability:ip", ip), exchangePerIP, throttleWindow); err != nil {
		return Guest{}, "", err
	}
	hash, ok := token.Hash(rawToken)
	if !ok || !validKind(kind) {
		return Guest{}, "", ErrTokenInvalid
	}
	g := Guest{Kind: kind}
	var generation int
	var capExpires *time.Time
	err := s.pool.QueryRow(ctx, q("capability_lookup"), hash, kind).
		Scan(&g.CapabilityID, &g.BranchID, &g.ResourceID, &generation, &capExpires)
	if errors.Is(err, pgx.ErrNoRows) {
		return Guest{}, "", ErrTokenInvalid
	}
	if err != nil {
		return Guest{}, "", fmt.Errorf("lookup capability: %w", err)
	}
	raw, sessionHash := token.New()
	if err := s.pool.QueryRow(ctx, q("guest_insert"), g.CapabilityID, generation, sessionHash, GuestSessionLifetime, capExpires).
		Scan(&g.SessionID, &g.ExpiresAt); err != nil {
		// A rotation between lookup and insert leaves a session of an old
		// generation, which never validates; no special handling needed.
		return Guest{}, "", fmt.Errorf("create guest session: %w", err)
	}
	return g, raw, nil
}

// AuthenticateGuest resolves a guest session cookie.
func (s *Service) AuthenticateGuest(ctx context.Context, raw string) (Guest, error) {
	hash, ok := token.Hash(raw)
	if !ok {
		return Guest{}, ErrUnauthenticated
	}
	var g Guest
	err := s.pool.QueryRow(ctx, q("guest_authenticate"), hash).
		Scan(&g.SessionID, &g.CapabilityID, &g.BranchID, &g.Kind, &g.ResourceID, &g.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Guest{}, ErrUnauthenticated
	}
	if err != nil {
		return Guest{}, fmt.Errorf("authenticate guest: %w", err)
	}
	return g, nil
}

// RevalidateGuest re-checks a guest inside a mutation transaction (FOR
// SHARE), so rotation/revocation either waits for the mutation or wins.
func RevalidateGuest(ctx context.Context, tx pgx.Tx, g Guest) (Guest, error) {
	out := Guest{SessionID: g.SessionID}
	err := tx.QueryRow(ctx, q("guest_revalidate"), g.SessionID).
		Scan(&out.CapabilityID, &out.BranchID, &out.Kind, &out.ResourceID, &out.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Guest{}, ErrUnauthenticated
	}
	return out, err
}

// StartAnonymous returns the existing anonymous session for raw when valid,
// or creates one (throttled per client address).
func (s *Service) StartAnonymous(ctx context.Context, raw string, ip netip.Addr) (Anonymous, string, bool, error) {
	if raw != "" {
		if a, err := s.AuthenticateAnonymous(ctx, raw); err == nil {
			return a, "", false, nil
		} else if !errors.Is(err, ErrUnauthenticated) {
			return Anonymous{}, "", false, err
		}
	}
	if err := s.limiter.Hit(ctx, throttle.IPKey("anonymous:ip", ip), anonymousPerIP, throttleWindow); err != nil {
		return Anonymous{}, "", false, err
	}
	newRaw, hash := token.New()
	var a Anonymous
	if err := s.pool.QueryRow(ctx, q("anonymous_insert"), hash, AnonymousSessionLifetime).Scan(&a.ID, &a.ExpiresAt); err != nil {
		return Anonymous{}, "", false, fmt.Errorf("create anonymous session: %w", err)
	}
	return a, newRaw, true, nil
}

// AuthenticateAnonymous resolves an anonymous session cookie.
func (s *Service) AuthenticateAnonymous(ctx context.Context, raw string) (Anonymous, error) {
	hash, ok := token.Hash(raw)
	if !ok {
		return Anonymous{}, ErrUnauthenticated
	}
	var a Anonymous
	err := s.pool.QueryRow(ctx, q("anonymous_authenticate"), hash).Scan(&a.ID, &a.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Anonymous{}, ErrUnauthenticated
	}
	if err != nil {
		return Anonymous{}, fmt.Errorf("authenticate anonymous: %w", err)
	}
	return a, nil
}

// Purge deletes long-expired guest and anonymous sessions.
func (s *Service) Purge(ctx context.Context) (int64, error) {
	var total int64
	for _, name := range []string{"purge_guest", "purge_anonymous"} {
		tag, err := s.pool.Exec(ctx, q(name))
		if err != nil {
			return total, fmt.Errorf("%s: %w", name, err)
		}
		total += tag.RowsAffected()
	}
	return total, nil
}

// ExpireCapability limits a capability's remaining life (e.g. a terminal
// queue ticket stays trackable briefly so the guest sees the outcome).
func ExpireCapability(ctx context.Context, tx pgx.Tx, kind, resourceID string, after time.Duration) error {
	_, err := tx.Exec(ctx, q("capability_expire"), kind, resourceID, after)
	return err
}
