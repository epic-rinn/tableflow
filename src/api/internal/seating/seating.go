// Package seating implements tables, seating groups, the queue, holds,
// visits and table claims. Queue, seating and table-lifecycle commands share
// one package because call/seat/move must lock tickets, tables and visits in
// one transaction.
//
// Lock order (data model): acting principal rows (FOR SHARE) → queue tickets
// → tables (ID order) → visits → capabilities/claims/audit (child rows).
// Queue joins and seating-group replacement take the branch row first (share
// / update) and lock nothing else, so they cannot form a cycle.
package seating

import (
	"context"
	"embed"
	"errors"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/epic-rinn/tableflow/src/api/internal/identity"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/throttle"
)

//go:embed sql/*.sql
var sqlFiles embed.FS

func q(name string) string {
	b, err := sqlFiles.ReadFile("sql/" + name + ".sql")
	if err != nil {
		panic("seating: missing SQL " + name)
	}
	return string(b)
}

// Needs a table can support and a party can require.
var allNeeds = []string{"accessible", "high_chair"}

// Limits.
const (
	maxParty          = 50
	maxTables         = 100
	boardPageDefault  = 50
	boardPageMax      = 100
	joinsPerSession   = 5
	joinsPerIP        = 30
	joinWindow        = 10 * time.Minute
	trackingAfterEnd  = 2 * time.Hour
	maxReasonRunes    = 500
	defaultJoinSource = "guest"
)

// Errors mapped to HTTP responses.
var (
	ErrNotFound          = errors.New("not found")
	ErrForbidden         = errors.New("forbidden")
	ErrVersionConflict   = errors.New("version conflict")
	ErrTableUnavailable  = errors.New("table unavailable")
	ErrTableIncompatible = errors.New("table incompatible")
	ErrTicketState       = errors.New("ticket state conflict")
	ErrVisitState        = errors.New("visit state conflict")
	ErrBypass            = errors.New("an older compatible party is waiting")
	ErrQueueActive       = errors.New("queue has active tickets")
	ErrPartyNeedsStaff   = errors.New("party needs staff assistance")
	ErrLabelTaken        = errors.New("table label taken")
	ErrTooManyTables     = errors.New("table limit reached")
)

// ValidationError carries per-field messages.
type ValidationError struct{ Fields map[string]string }

func (e *ValidationError) Error() string { return "validation failed" }

// Service implements seating use cases.
type Service struct {
	pool    *pgxpool.Pool
	staff   *identity.Service
	limiter *throttle.Limiter
}

// NewService builds the service; staff re-validates acting staff sessions.
func NewService(pool *pgxpool.Pool, staff *identity.Service) *Service {
	return &Service{pool: pool, staff: staff, limiter: throttle.New(pool)}
}

// normalizeNeeds validates and sorts a needs list.
func normalizeNeeds(in []string) ([]string, bool) {
	out := []string{}
	for _, n := range in {
		if !slices.Contains(allNeeds, n) {
			return nil, false
		}
		if !slices.Contains(out, n) {
			out = append(out, n)
		}
	}
	slices.Sort(out)
	return out, true
}

// compatible: the table seats the party and supports every need.
func compatible(capacity int, tableNeeds []string, party int, needs []string) bool {
	if party > capacity {
		return false
	}
	for _, n := range needs {
		if !slices.Contains(tableNeeds, n) {
			return false
		}
	}
	return true
}

// staffActor re-validates a staff principal in tx and checks its branch and
// that it holds one of roles. Other branches look like "not found".
func (s *Service) staffActor(ctx context.Context, tx pgx.Tx, p identity.Principal, branchID string, roles ...string) (identity.Principal, error) {
	actor, err := s.staff.Revalidate(ctx, tx, p)
	if err != nil {
		return identity.Principal{}, err
	}
	if actor.BranchID != branchID {
		return identity.Principal{}, ErrNotFound
	}
	for _, r := range roles {
		if actor.Has(r) {
			return actor, nil
		}
	}
	return identity.Principal{}, ErrForbidden
}

// execOne executes a statement that must affect exactly one row.
func execOne(ctx context.Context, tx pgx.Tx, name string, args ...any) error {
	tag, err := tx.Exec(ctx, q(name), args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrVersionConflict
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505"
}

// validReason trims and bounds an override/close reason.
func validReason(r string) (string, bool) {
	n := len([]rune(r))
	return r, n >= 1 && n <= maxReasonRunes
}
