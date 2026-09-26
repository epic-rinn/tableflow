// Package billing implements charge policies, bill calculation, cashier
// settlement, receipts and full refunds (MVP-11/12/13).
//
// Lock order: acting staff rows (FOR SHARE) → visit (FOR UPDATE; the same row orders and cancellations lock) → guest
// capability/sessions (children of the visit) → settlement → refund.
package billing

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/epic-rinn/tableflow/src/api/internal/access"
	"github.com/epic-rinn/tableflow/src/api/internal/identity"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/audit"
)

//go:embed sql/*.sql
var sqlFiles embed.FS

func q(name string) string {
	b, err := sqlFiles.ReadFile("sql/" + name + ".sql")
	if err != nil {
		panic("billing: missing SQL " + name)
	}
	return string(b)
}

// Errors mapped to HTTP responses.
var (
	ErrNotFound        = errors.New("not found")
	ErrForbidden       = errors.New("forbidden")
	ErrVersionConflict = errors.New("version conflict")
	ErrVisitState      = errors.New("visit state conflict")
	ErrAmountMismatch  = errors.New("amount differs from bill total")
	ErrNothingToSettle = errors.New("no chargeable lines")
)

// ValidationError carries per-field messages.
type ValidationError struct{ Fields map[string]string }

func (e *ValidationError) Error() string { return "validation failed" }

// BillConflictError: the observed bill version is stale; carries the fresh bill (BIL-A1).
type BillConflictError struct{ Bill Bill }

func (e *BillConflictError) Error() string { return "bill version conflict" }

// UnresolvedError lists lines that must be served, rejected or cancelled
// before settlement can begin (BIL-002).
type UnresolvedError struct{ Lines []BillLine }

func (e *UnresolvedError) Error() string { return "unresolved lines" }

// AlreadyPaidError: the visit already has a settlement (BIL-A3).
type AlreadyPaidError struct{ Settlement SettlementRef }

func (e *AlreadyPaidError) Error() string { return "already paid" }

// AlreadyRefundedError: the settlement already has its one refund.
type AlreadyRefundedError struct{ RefundID string }

func (e *AlreadyRefundedError) Error() string { return "already refunded" }

// Roles allowed to read bills and settle; refunds are manager-only.
var cashierRoles = []string{identity.RoleCashier, identity.RoleManager}

// Service implements billing use cases.
type Service struct {
	pool  *pgxpool.Pool
	staff *identity.Service
}

// NewService builds the service.
func NewService(pool *pgxpool.Pool, staff *identity.Service) *Service {
	return &Service{pool: pool, staff: staff}
}

type querier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

// revalidate re-reads the acting staff inside tx and checks roles.
func (s *Service) revalidate(ctx context.Context, tx pgx.Tx, p identity.Principal, roles []string) (identity.Principal, error) {
	p, err := s.staff.Revalidate(ctx, tx, p)
	if err != nil {
		return identity.Principal{}, err
	}
	if !slices.ContainsFunc(roles, p.Has) {
		return identity.Principal{}, ErrForbidden
	}
	return p, nil
}

// --- charge policy ---------------------------------------------------------

// PolicyView is the current policy with its creation time.
type PolicyView struct {
	Policy
	UpdatedAt *time.Time `json:"updated_at"`
}

func currentPolicy(ctx context.Context, db querier, branchID string) (PolicyView, error) {
	v := PolicyView{Policy: Policy{TaxMode: TaxExclusive}}
	var at time.Time
	err := db.QueryRow(ctx, q("policy_current"), branchID).Scan(&v.Version, &v.TaxMode, &v.TaxBP, &v.ServiceBP, &at)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, nil
	}
	if err != nil {
		return PolicyView{}, err
	}
	v.Configured, v.UpdatedAt = true, &at
	return v, nil
}

// ChargePolicy returns the branch's current policy to its staff.
func (s *Service) ChargePolicy(ctx context.Context, p identity.Principal, branchID string) (PolicyView, error) {
	if p.BranchID != branchID {
		return PolicyView{}, ErrNotFound
	}
	return currentPolicy(ctx, s.pool, branchID)
}

// PolicyIn is a manager's policy edit.
type PolicyIn struct {
	ExpectedVersion *int   `json:"expected_version"`
	TaxMode         string `json:"tax_mode"`
	TaxBP           *int   `json:"tax_bp"`
	ServiceBP       *int   `json:"service_bp"`
}

func validatePolicy(in PolicyIn) error {
	f := map[string]string{}
	if in.ExpectedVersion == nil || *in.ExpectedVersion < 0 {
		f["expected_version"] = "is required"
	}
	if in.TaxMode != TaxExclusive && in.TaxMode != TaxInclusive {
		f["tax_mode"] = "must be exclusive or inclusive"
	}
	if in.TaxBP == nil || *in.TaxBP < 0 || *in.TaxBP > 10000 {
		f["tax_bp"] = "must be 0–10000 basis points"
	}
	if in.ServiceBP == nil || *in.ServiceBP < 0 || *in.ServiceBP > 10000 {
		f["service_bp"] = "must be 0–10000 basis points"
	}
	if len(f) > 0 {
		return &ValidationError{Fields: f}
	}
	return nil
}

// SetChargePolicy appends a new policy version (manager). Open bills show
// it at once; frozen snapshots and settlements never change.
func (s *Service) SetChargePolicy(ctx context.Context, tx pgx.Tx, p identity.Principal, branchID string, in PolicyIn, requestID string) (PolicyView, error) {
	if err := validatePolicy(in); err != nil {
		return PolicyView{}, err
	}
	p, err := s.revalidate(ctx, tx, p, []string{identity.RoleManager})
	if err != nil {
		return PolicyView{}, err
	}
	if p.BranchID != branchID {
		return PolicyView{}, ErrNotFound
	}
	// No branch lock: staff administration locks branch → staff, so taking
	// the branch after the actor rows could deadlock. The (branch, version)
	// primary key arbitrates concurrent edits instead.
	cur, err := currentPolicy(ctx, tx, branchID)
	if err != nil {
		return PolicyView{}, err
	}
	if cur.Version != *in.ExpectedVersion {
		return PolicyView{}, ErrVersionConflict
	}
	next := PolicyView{Policy: Policy{Version: cur.Version + 1, TaxMode: in.TaxMode, TaxBP: *in.TaxBP, ServiceBP: *in.ServiceBP, Configured: true}}
	var at time.Time
	if err := tx.QueryRow(ctx, q("policy_insert"), branchID, next.Version, next.TaxMode, next.TaxBP, next.ServiceBP, p.StaffID).Scan(&at); err != nil {
		if pg := (*pgconn.PgError)(nil); errors.As(err, &pg) && pg.Code == "23505" { // concurrent edit took this version
			return PolicyView{}, ErrVersionConflict
		}
		return PolicyView{}, err
	}
	next.UpdatedAt = &at
	err = audit.Record(ctx, tx, audit.Event{BranchID: branchID, ActorStaffID: &p.StaffID, Action: "charge_policy.updated",
		ResourceType: "charge_policy", ResourceID: branchID, RequestID: requestID,
		Details: map[string]any{"version": next.Version, "tax_mode": next.TaxMode, "tax_bp": next.TaxBP, "service_bp": next.ServiceBP}})
	return next, err
}

// --- bill ------------------------------------------------------------------

// BillLine is one chargeable line (or an unresolved one in a conflict).
type BillLine struct {
	ID              string          `json:"id"`
	NameTH          string          `json:"name_th"`
	NameEN          string          `json:"name_en"`
	Options         json.RawMessage `json:"options"`
	UnitPriceSatang int64           `json:"unit_price_satang"`
	Quantity        int             `json:"quantity"`
	LineTotalSatang int64           `json:"line_total_satang"`
	State           string          `json:"state"`
}

// SettlementRef identifies a visit's settlement.
type SettlementRef struct {
	ID               string    `json:"id"`
	ReceiptReference string    `json:"receipt_reference"`
	PaidAt           time.Time `json:"paid_at"`
}

// Bill is the itemised bill of one visit (BIL-001/003).
type Bill struct {
	VisitID     string `json:"visit_id"`
	BranchID    string `json:"branch_id"`
	TableLabel  string `json:"table_label"`
	VisitState  string `json:"visit_state"`
	BillVersion int    `json:"bill_version"`
	// Frozen: totals come from the settlement snapshot, not live lines.
	Frozen          bool           `json:"frozen"`
	Policy          Policy         `json:"policy"`
	Lines           []BillLine     `json:"lines"`
	UnresolvedLines int            `json:"unresolved_lines"`
	Settlement      *SettlementRef `json:"settlement"`
	// Claim status only; staff also see a masked email and the tier (LOY-001).
	MemberClaim *ClaimView `json:"member_claim"`
	ServerTime  time.Time `json:"server_time"`
	Totals
}

// ClaimView is what a bill reveals about a member claim.
type ClaimView struct {
	Claimed     bool    `json:"claimed"`
	MaskedEmail *string `json:"masked_email,omitempty"`
	Tier        *string `json:"tier,omitempty"`
}

type visitRow struct {
	branchID, state, label string
	billVersion, version   int
	memberID               *string
}

func scanVisit(row pgx.Row) (visitRow, error) {
	var v visitRow
	err := row.Scan(&v.branchID, &v.state, &v.billVersion, &v.version, &v.label, &v.memberID)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, ErrNotFound
	}
	return v, err
}

func chargeable(state string) bool { return state != "rejected" && state != "cancelled" }

// liveLines reads a visit's lines: chargeable ones and the unresolved subset.
func liveLines(ctx context.Context, db querier, visitID string) (lines, unresolved []BillLine, gross int64, err error) {
	rows, err := db.Query(ctx, q("bill_lines"), visitID)
	if err != nil {
		return nil, nil, 0, err
	}
	all, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (BillLine, error) {
		var l BillLine
		err := r.Scan(&l.ID, &l.NameTH, &l.NameEN, &l.Options, &l.UnitPriceSatang, &l.Quantity, &l.State)
		l.LineTotalSatang = l.UnitPriceSatang * int64(l.Quantity)
		return l, err
	})
	if err != nil {
		return nil, nil, 0, err
	}
	lines = []BillLine{}
	for _, l := range all {
		if !chargeable(l.State) {
			continue
		}
		lines = append(lines, l)
		gross += l.LineTotalSatang
		if l.State != "served" {
			unresolved = append(unresolved, l)
		}
	}
	return lines, unresolved, gross, nil
}

// snapshot is a frozen bill row.
type snapshot struct {
	id     string
	policy Policy
	totals Totals
	lines  []BillLine
	// Member benefit frozen at begin (nil for non-members).
	memberID       *string
	tier           *string
	loyaltyVersion *int
	satangPerPoint *int64
}

func loadSnapshot(ctx context.Context, db querier, visitID string, billVersion int) (snapshot, error) {
	var sn snapshot
	var raw []byte
	t := &sn.totals
	err := db.QueryRow(ctx, q("snapshot_get"), visitID, billVersion).Scan(&sn.id, &sn.policy.Version, &sn.policy.TaxMode,
		&sn.policy.TaxBP, &sn.policy.ServiceBP, &t.DiscountBP, &t.GrossSatang, &t.DiscountSatang, &t.ServiceSatang,
		&t.TaxSatang, &t.TotalSatang, &raw, &sn.memberID, &sn.tier, &sn.loyaltyVersion, &sn.satangPerPoint)
	if err != nil {
		return sn, err
	}
	sn.policy.Configured = sn.policy.Version > 0
	t.NetSatang = t.GrossSatang - t.DiscountSatang
	err = json.Unmarshal(raw, &sn.lines)
	return sn, err
}

// buildBill assembles the bill for a visit row: frozen for settling, paid
// and departed visits; live (current policy) otherwise.
func buildBill(ctx context.Context, db querier, visitID string, v visitRow) (Bill, error) {
	b := Bill{VisitID: visitID, BranchID: v.branchID, TableLabel: v.label, VisitState: v.state,
		BillVersion: v.billVersion, ServerTime: time.Now().UTC()}
	switch v.state {
	case "settling", "paid", "departed":
		sn, err := loadSnapshot(ctx, db, visitID, v.billVersion)
		if err != nil {
			return Bill{}, err
		}
		b.Frozen, b.Policy, b.Totals, b.Lines = true, sn.policy, sn.totals, sn.lines
		if sn.memberID != nil {
			b.MemberClaim = &ClaimView{Claimed: true, Tier: sn.tier}
		}
		if v.state != "settling" {
			ref := SettlementRef{}
			if err := db.QueryRow(ctx, q("settlement_for_visit"), visitID).Scan(&ref.ID, &ref.ReceiptReference, &ref.PaidAt); err != nil {
				return Bill{}, err
			}
			b.Settlement = &ref
		}
		return b, nil
	}
	lines, unresolved, gross, err := liveLines(ctx, db, visitID)
	if err != nil {
		return Bill{}, err
	}
	pol, err := currentPolicy(ctx, db, v.branchID)
	if err != nil {
		return Bill{}, err
	}
	// A claimed open visit previews the member's current tier discount; the
	// benefit is fixed only when settlement begins (LOY-003).
	discount := 0
	if v.memberID != nil {
		tier, err := memberTier(ctx, db, v.branchID, *v.memberID)
		if err != nil {
			return Bill{}, err
		}
		lp, err := currentLoyalty(ctx, db, v.branchID)
		if err != nil {
			return Bill{}, err
		}
		discount = lp.DiscountBP(tier)
		b.MemberClaim = &ClaimView{Claimed: true, Tier: &tier}
	}
	b.Policy, b.Lines, b.UnresolvedLines = pol.Policy, lines, len(unresolved)
	b.Totals = Calculate(gross, pol.Policy, discount)
	return b, nil
}

// Reader is who reads a bill: branch cashier/manager staff or the visit's guest.
type Reader struct {
	Staff *identity.Principal
	Guest *access.Guest
}

// Bill returns the itemised bill of visitID.
func (s *Service) Bill(ctx context.Context, rd Reader, visitID string) (Bill, error) {
	v, err := scanVisit(s.pool.QueryRow(ctx, q("visit_get"), visitID))
	if err != nil {
		return Bill{}, err
	}
	switch {
	case rd.Staff != nil:
		if rd.Staff.BranchID != v.branchID {
			return Bill{}, ErrNotFound
		}
		if !slices.ContainsFunc(cashierRoles, rd.Staff.Has) {
			return Bill{}, ErrForbidden
		}
	case rd.Guest != nil:
		if rd.Guest.Kind != access.KindVisit || rd.Guest.ResourceID != visitID {
			return Bill{}, ErrNotFound
		}
	default:
		return Bill{}, ErrForbidden
	}
	b, err := buildBill(ctx, s.pool, visitID, v)
	if err != nil || b.MemberClaim == nil {
		return b, err
	}
	// Shared QR access learns only that the visit is claimed (LOY-001).
	if rd.Staff == nil {
		b.MemberClaim = &ClaimView{Claimed: true}
		return b, nil
	}
	if v.memberID != nil {
		var email string
		if err := s.pool.QueryRow(ctx, q("member_email"), *v.memberID).Scan(&email); err != nil {
			return Bill{}, err
		}
		masked := maskEmail(email)
		b.MemberClaim.MaskedEmail = &masked
	}
	return b, nil
}

// Resolve finds a bill from the diner's dining QR token (cashier, BIL-001).
// The token identifies the visit only; it never authorizes settlement.
func (s *Service) Resolve(ctx context.Context, p identity.Principal, rawToken string) (Bill, error) {
	if !slices.ContainsFunc(cashierRoles, p.Has) {
		return Bill{}, ErrForbidden
	}
	branch, visitID, err := access.ResolveCapability(ctx, s.pool, access.KindVisit, rawToken)
	if errors.Is(err, access.ErrTokenInvalid) || (err == nil && branch != p.BranchID) {
		return Bill{}, ErrNotFound
	}
	if err != nil {
		return Bill{}, err
	}
	return s.Bill(ctx, Reader{Staff: &p}, visitID)
}
