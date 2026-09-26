package billing

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"regexp"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/epic-rinn/tableflow/src/api/internal/identity"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/audit"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/bizdate"
)

// Receipt page bounds.
const (
	pageDefault = 25
	pageMax     = 100
)

var receiptPattern = regexp.MustCompile(`^R-[A-Z2-7]{10}$`)

// Refund is the one full refund of a settlement.
type Refund struct {
	ID                string    `json:"id"`
	AmountSatang      int64     `json:"amount_satang"`
	Reason            string    `json:"reason"`
	ExternalReference string    `json:"external_reference"`
	RecordedBy        string    `json:"recorded_by"`
	CreatedAt         time.Time `json:"created_at"`
}

// Receipt is an immutable historical settlement with its frozen bill (BIL-008).
type Receipt struct {
	ID                string          `json:"id"`
	VisitID           string          `json:"visit_id"`
	ReceiptReference  string          `json:"receipt_reference"`
	TableLabel        string          `json:"table_label"`
	VisitState        string          `json:"visit_state"`
	AmountSatang      int64           `json:"amount_satang"`
	Method            string          `json:"method"`
	VerificationNote  string          `json:"verification_note"`
	ExternalReference *string         `json:"external_reference"`
	ConfirmedBy       string          `json:"confirmed_by"`
	PaidAt            time.Time       `json:"paid_at"`
	BillVersion       int             `json:"bill_version"`
	Policy            Policy          `json:"policy"`
	Lines             json.RawMessage `json:"lines"`
	Refund            *Refund         `json:"refund"`
	Member            *ReceiptMember  `json:"member"`
	Totals
}

// ReceiptMember is the loyalty outcome recorded with a member settlement.
type ReceiptMember struct {
	Tier           string `json:"tier"`
	PointsEarned   int64  `json:"points_earned"`
	EligibleSatang int64  `json:"eligible_satang"`
}

func loadReceipt(ctx context.Context, db querier, id string) (Receipt, string, error) {
	var rc Receipt
	var branch string
	var rid, rreason, rref, rby *string
	var ramount *int64
	var rat *time.Time
	var isMember bool
	var mPoints, mEligible *int64
	var mTier *string
	t := &rc.Totals
	err := db.QueryRow(ctx, q("receipt_get"), id).Scan(&rc.ID, &branch, &rc.VisitID, &rc.ReceiptReference, &rc.AmountSatang,
		&rc.Method, &rc.VerificationNote, &rc.ExternalReference, &rc.PaidAt, &rc.ConfirmedBy, &rc.TableLabel, &rc.VisitState,
		&rc.BillVersion, &rc.Policy.Version, &rc.Policy.TaxMode, &rc.Policy.TaxBP, &rc.Policy.ServiceBP, &t.DiscountBP,
		&t.GrossSatang, &t.DiscountSatang, &t.ServiceSatang, &t.TaxSatang, &t.TotalSatang, &rc.Lines,
		&rid, &ramount, &rreason, &rref, &rby, &rat, &isMember, &mPoints, &mEligible, &mTier)
	if errors.Is(err, pgx.ErrNoRows) {
		return rc, "", ErrNotFound
	}
	if err != nil {
		return rc, "", err
	}
	rc.Policy.Configured = rc.Policy.Version > 0
	t.NetSatang = t.GrossSatang - t.DiscountSatang
	if isMember && mPoints != nil && mEligible != nil && mTier != nil {
		rc.Member = &ReceiptMember{Tier: *mTier, PointsEarned: *mPoints, EligibleSatang: *mEligible}
	}
	if rid != nil {
		rc.Refund = &Refund{ID: *rid, AmountSatang: *ramount, Reason: *rreason, ExternalReference: *rref, RecordedBy: *rby, CreatedAt: *rat}
	}
	return rc, branch, nil
}

// Receipt returns one historical receipt to branch cashiers/managers.
// It never reactivates guest access (BIL-008).
func (s *Service) Receipt(ctx context.Context, p identity.Principal, id string) (Receipt, error) {
	if !slices.ContainsFunc(cashierRoles, p.Has) {
		return Receipt{}, ErrForbidden
	}
	rc, branch, err := loadReceipt(ctx, s.pool, id)
	if err != nil {
		return Receipt{}, err
	}
	if branch != p.BranchID {
		return Receipt{}, ErrNotFound
	}
	return rc, nil
}

// ReceiptSummary is one row of the receipt list.
type ReceiptSummary struct {
	ID               string    `json:"id"`
	ReceiptReference string    `json:"receipt_reference"`
	AmountSatang     int64     `json:"amount_satang"`
	Method           string    `json:"method"`
	PaidAt           time.Time `json:"paid_at"`
	TableLabel       string    `json:"table_label"`
	Refunded         bool      `json:"refunded"`
}

// ReceiptPage is a keyset page, newest first.
type ReceiptPage struct {
	Items      []ReceiptSummary `json:"items"`
	NextCursor *string          `json:"next_cursor"`
}

type cursor struct {
	T  time.Time `json:"t"`
	ID string    `json:"i"`
	B  string    `json:"b"`
}

// Receipts lists a branch's settlements, optionally one receipt reference.
func (s *Service) Receipts(ctx context.Context, p identity.Principal, branchID, reference, from, to, after string, limit int) (ReceiptPage, error) {
	if p.BranchID != branchID {
		return ReceiptPage{}, ErrNotFound
	}
	if !slices.ContainsFunc(cashierRoles, p.Has) {
		return ReceiptPage{}, ErrForbidden
	}
	var ref *string
	if reference != "" {
		if !receiptPattern.MatchString(reference) {
			return ReceiptPage{}, &ValidationError{Fields: map[string]string{"receipt_reference": "must look like R-XXXXXXXXXX"}}
		}
		ref = &reference
	}
	// Optional report drill-down range in the branch's business dates.
	var start, end *time.Time
	if from != "" || to != "" {
		var tz string
		if err := s.pool.QueryRow(ctx, q("branch_timezone"), branchID).Scan(&tz); err != nil {
			return ReceiptPage{}, err
		}
		r, err := bizdate.Parse(tz, from, to)
		if err != nil {
			return ReceiptPage{}, &ValidationError{Fields: map[string]string{"from": bizdate.ErrRange.Error()}}
		}
		start, end = &r.Start, &r.End
	}
	if limit <= 0 {
		limit = pageDefault
	}
	limit = min(limit, pageMax)
	c := cursor{T: time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC), ID: "ffffffff-ffff-ffff-ffff-ffffffffffff", B: branchID}
	if after != "" {
		b, err := base64.RawURLEncoding.DecodeString(after)
		if err != nil || json.Unmarshal(b, &c) != nil || c.B != branchID {
			return ReceiptPage{}, &ValidationError{Fields: map[string]string{"cursor": "invalid cursor"}}
		}
	}
	rows, err := s.pool.Query(ctx, q("receipts_page"), branchID, c.T, c.ID, ref, limit+1, start, end)
	if err != nil {
		return ReceiptPage{}, err
	}
	items, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (ReceiptSummary, error) {
		var it ReceiptSummary
		return it, r.Scan(&it.ID, &it.ReceiptReference, &it.AmountSatang, &it.Method, &it.PaidAt, &it.TableLabel, &it.Refunded)
	})
	if err != nil {
		return ReceiptPage{}, err
	}
	page := ReceiptPage{Items: items}
	if len(items) > limit {
		page.Items = items[:limit]
		last := page.Items[limit-1]
		b, _ := json.Marshal(cursor{T: last.PaidAt, ID: last.ID, B: branchID})
		next := base64.RawURLEncoding.EncodeToString(b)
		page.NextCursor = &next
	}
	return page, nil
}

// RefundIn is a manager's full refund record (BIL-007).
type RefundIn struct {
	Reason            string `json:"reason"`
	ExternalReference string `json:"external_reference"`
}

// RecordRefund records the one full refund of a settlement (manager). The
// settlement and visit are never modified or reopened.
func (s *Service) RecordRefund(ctx context.Context, tx pgx.Tx, p identity.Principal, settlementID string, in RefundIn, requestID string) (Receipt, error) {
	f := map[string]string{}
	reason, ok := validText(in.Reason, 500)
	if !ok {
		f["reason"] = "must be 1–500 characters"
	}
	ref, ok := validText(in.ExternalReference, 200)
	if !ok {
		f["external_reference"] = "must be 1–200 characters"
	}
	if len(f) > 0 {
		return Receipt{}, &ValidationError{Fields: f}
	}
	p, err := s.revalidate(ctx, tx, p, []string{identity.RoleManager})
	if err != nil {
		return Receipt{}, err
	}
	var visitID string
	var memberID *string
	err = tx.QueryRow(ctx, q("settlement_visit"), settlementID).Scan(&visitID, &memberID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Receipt{}, ErrNotFound
	}
	if err != nil {
		return Receipt{}, err
	}
	v, err := scanVisit(tx.QueryRow(ctx, q("visit_lock"), visitID))
	if err != nil {
		return Receipt{}, err
	}
	if v.branchID != p.BranchID {
		return Receipt{}, ErrNotFound
	}
	// Lock order: visit → member profile → settlement → refund (data model).
	var prof profileRow
	if memberID != nil {
		if prof, err = lockProfile(ctx, tx, v.branchID, *memberID); err != nil {
			return Receipt{}, err
		}
	}
	var branch string
	var amount int64
	var sMember *string
	var eligible, points *int64
	var loyaltyVersion *int
	if err := tx.QueryRow(ctx, q("settlement_lock"), settlementID).Scan(&branch, &amount, &sMember, &eligible, &points, &loyaltyVersion); err != nil {
		return Receipt{}, err
	}
	var existing string
	err = tx.QueryRow(ctx, q("refund_get"), settlementID).Scan(&existing)
	if err == nil {
		return Receipt{}, &AlreadyRefundedError{RefundID: existing}
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Receipt{}, err
	}
	var id string
	if err := tx.QueryRow(ctx, q("refund_insert"), branch, settlementID, amount, reason, ref, p.StaffID).Scan(&id); err != nil {
		return Receipt{}, err
	}
	// The original award is reversed exactly once with the original amounts
	// and policy version, never today's rates (LOY-006); the unique
	// (settlement, kind) key backs the refund's own uniqueness.
	if sMember != nil {
		if err := applyLedger(ctx, tx, branch, *sMember, settlementID, "reversal", prof, -*points, -*eligible, *loyaltyVersion); err != nil {
			return Receipt{}, err
		}
	}
	if err := audit.Record(ctx, tx, audit.Event{BranchID: branch, ActorStaffID: &p.StaffID, Action: "settlement.refunded",
		ResourceType: "settlement", ResourceID: settlementID, Reason: &reason, RequestID: requestID,
		Details: map[string]any{"refund_id": id, "amount_satang": amount, "external_reference": ref}}); err != nil {
		return Receipt{}, err
	}
	rc, _, err := loadReceipt(ctx, tx, settlementID)
	return rc, err
}
