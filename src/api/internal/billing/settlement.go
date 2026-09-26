package billing

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/epic-rinn/tableflow/src/api/internal/access"
	"github.com/epic-rinn/tableflow/src/api/internal/identity"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/audit"
)

// Payment methods (BIL-005).
var methods = []string{"cash", "bank_transfer", "card", "other"}

func validText(s string, max int) (string, bool) {
	s = strings.TrimSpace(s)
	n := utf8.RuneCountInString(s)
	return s, n >= 1 && n <= max
}

// lockVisit re-validates the cashier, then locks the visit row.
func (s *Service) lockVisit(ctx context.Context, tx pgx.Tx, p identity.Principal, visitID string) (identity.Principal, visitRow, error) {
	p, err := s.revalidate(ctx, tx, p, cashierRoles)
	if err != nil {
		return p, visitRow{}, err
	}
	v, err := scanVisit(tx.QueryRow(ctx, q("visit_lock"), visitID))
	if err != nil {
		return p, v, err
	}
	if v.branchID != p.BranchID {
		return p, v, ErrNotFound
	}
	return p, v, nil
}

// paidError reports an existing settlement for paid/departed visits.
func paidError(ctx context.Context, tx pgx.Tx, visitID string) error {
	var ref SettlementRef
	if err := tx.QueryRow(ctx, q("settlement_for_visit"), visitID).Scan(&ref.ID, &ref.ReceiptReference, &ref.PaidAt); err != nil {
		return err
	}
	return &AlreadyPaidError{Settlement: ref}
}

// checkState maps a non-expected visit state to its error.
func checkState(ctx context.Context, tx pgx.Tx, visitID string, v visitRow, want string) error {
	switch {
	case v.state == want:
		return nil
	case v.state == "paid" || v.state == "departed":
		return paidError(ctx, tx, visitID)
	default:
		return ErrVisitState
	}
}

// BeginSettlement freezes ordering and snapshots totals at the observed
// bill version (BIL-002). Unserved chargeable lines block it.
func (s *Service) BeginSettlement(ctx context.Context, tx pgx.Tx, p identity.Principal, visitID string, expected int, requestID string) (Bill, error) {
	p, v, err := s.lockVisit(ctx, tx, p, visitID)
	if err != nil {
		return Bill{}, err
	}
	if err := checkState(ctx, tx, visitID, v, "open"); err != nil {
		return Bill{}, err
	}
	if v.billVersion != expected {
		fresh, err := buildBill(ctx, tx, visitID, v)
		if err != nil {
			return Bill{}, err
		}
		return Bill{}, &BillConflictError{Bill: fresh}
	}
	lines, unresolved, gross, err := liveLines(ctx, tx, visitID)
	if err != nil {
		return Bill{}, err
	}
	if len(unresolved) > 0 {
		return Bill{}, &UnresolvedError{Lines: unresolved}
	}
	if len(lines) == 0 {
		return Bill{}, ErrNothingToSettle // close-empty handles visits without charges (SEA-004)
	}
	pol, err := currentPolicy(ctx, tx, v.branchID)
	if err != nil {
		return Bill{}, err
	}
	// A claimed visit snapshots the member's tier benefit after locking the
	// profile (LOY-003); a tier earned by this bill applies to later bills.
	discount := 0
	var tier *string
	var loyaltyVersion *int
	var satangPerPoint *int64
	if v.memberID != nil {
		prof, err := lockProfile(ctx, tx, v.branchID, *v.memberID)
		if err != nil {
			return Bill{}, err
		}
		lp, err := currentLoyalty(ctx, tx, v.branchID)
		if err != nil {
			return Bill{}, err
		}
		discount, tier, loyaltyVersion, satangPerPoint = lp.DiscountBP(prof.tier), &prof.tier, &lp.Version, &lp.SatangPerPoint
	}
	t := Calculate(gross, pol.Policy, discount)
	if err := tx.QueryRow(ctx, q("visit_set_state"), visitID, "settling", true).Scan(&v.billVersion, &v.version); err != nil {
		return Bill{}, err
	}
	v.state = "settling"
	raw, err := json.Marshal(lines)
	if err != nil {
		return Bill{}, err
	}
	var snapID string
	if err := tx.QueryRow(ctx, q("snapshot_insert"), v.branchID, visitID, v.billVersion, pol.Version, pol.TaxMode, pol.TaxBP,
		pol.ServiceBP, t.DiscountBP, t.GrossSatang, t.DiscountSatang, t.ServiceSatang, t.TaxSatang, t.TotalSatang, raw, p.StaffID,
		v.memberID, tier, loyaltyVersion, satangPerPoint).
		Scan(&snapID); err != nil {
		return Bill{}, err
	}
	if err := audit.Record(ctx, tx, audit.Event{BranchID: v.branchID, ActorStaffID: &p.StaffID, Action: "settlement.begun",
		ResourceType: "visit", ResourceID: visitID, RequestID: requestID,
		Details: map[string]any{"bill_version": v.billVersion, "snapshot_id": snapID, "total_satang": t.TotalSatang}}); err != nil {
		return Bill{}, err
	}
	return buildBill(ctx, tx, visitID, v)
}

// ReopenSettlement returns a settling visit to open with a reason,
// invalidating its snapshot (BIL-007). Paid bills never reopen.
func (s *Service) ReopenSettlement(ctx context.Context, tx pgx.Tx, p identity.Principal, visitID string, expected int, reason, requestID string) (Bill, error) {
	r, ok := validText(reason, 500)
	if !ok {
		return Bill{}, &ValidationError{Fields: map[string]string{"reason": "must be 1–500 characters"}}
	}
	p, v, err := s.lockVisit(ctx, tx, p, visitID)
	if err != nil {
		return Bill{}, err
	}
	if err := checkState(ctx, tx, visitID, v, "settling"); err != nil {
		return Bill{}, err
	}
	if v.billVersion != expected {
		fresh, err := buildBill(ctx, tx, visitID, v)
		if err != nil {
			return Bill{}, err
		}
		return Bill{}, &BillConflictError{Bill: fresh}
	}
	if err := tx.QueryRow(ctx, q("visit_set_state"), visitID, "open", true).Scan(&v.billVersion, &v.version); err != nil {
		return Bill{}, err
	}
	v.state = "open"
	if err := audit.Record(ctx, tx, audit.Event{BranchID: v.branchID, ActorStaffID: &p.StaffID, Action: "settlement.reopened",
		ResourceType: "visit", ResourceID: visitID, Reason: &r, RequestID: requestID,
		Details: map[string]any{"bill_version": v.billVersion}}); err != nil {
		return Bill{}, err
	}
	return buildBill(ctx, tx, visitID, v)
}

// ConfirmIn is a cashier's payment confirmation (BIL-005).
type ConfirmIn struct {
	ExpectedVersion   *int    `json:"expected_version"`
	AmountSatang      *int64  `json:"amount_satang"`
	Method            string  `json:"method"`
	VerificationNote  string  `json:"verification_note"`
	ExternalReference *string `json:"external_reference"`
}

func (in *ConfirmIn) validate() error {
	f := map[string]string{}
	if in.ExpectedVersion == nil {
		f["expected_version"] = "is required"
	}
	if in.AmountSatang == nil || *in.AmountSatang < 0 {
		f["amount_satang"] = "is required"
	}
	valid := false
	for _, m := range methods {
		valid = valid || in.Method == m
	}
	if !valid {
		f["method"] = "must be cash, bank_transfer, card or other"
	}
	note, ok := validText(in.VerificationNote, 500)
	if !ok {
		f["verification_note"] = "must be 1–500 characters"
	}
	in.VerificationNote = note
	if in.ExternalReference != nil {
		ref, ok := validText(*in.ExternalReference, 200)
		switch {
		case strings.TrimSpace(*in.ExternalReference) == "":
			in.ExternalReference = nil
		case !ok:
			f["external_reference"] = "must be at most 200 characters"
		default:
			in.ExternalReference = &ref
		}
	}
	if len(f) > 0 {
		return &ValidationError{Fields: f}
	}
	return nil
}

// Settlement is the confirmation result.
type Settlement struct {
	ID               string    `json:"id"`
	VisitID          string    `json:"visit_id"`
	ReceiptReference string    `json:"receipt_reference"`
	AmountSatang     int64     `json:"amount_satang"`
	Method           string    `json:"method"`
	PaidAt           time.Time `json:"paid_at"`
	PointsEarned     *int64    `json:"points_earned"`
	Bill             Bill      `json:"bill"`
}

var receiptEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// receiptReference is an opaque, unguessable reference (BIL-008): 50 bits.
func receiptReference() string {
	b := make([]byte, 7)
	_, _ = rand.Read(b)
	return "R-" + receiptEncoding.EncodeToString(b)[:10]
}

// ConfirmSettlement records the exact payment, marks the visit paid and
// revokes dining access in one transaction (BIL-005/006). The table stays
// claimed until departure (SEA-004).
func (s *Service) ConfirmSettlement(ctx context.Context, tx pgx.Tx, p identity.Principal, visitID string, in ConfirmIn, requestID string) (Settlement, error) {
	if err := in.validate(); err != nil {
		return Settlement{}, err
	}
	p, v, err := s.lockVisit(ctx, tx, p, visitID)
	if err != nil {
		return Settlement{}, err
	}
	if err := checkState(ctx, tx, visitID, v, "settling"); err != nil {
		return Settlement{}, err
	}
	if v.billVersion != *in.ExpectedVersion {
		fresh, err := buildBill(ctx, tx, visitID, v)
		if err != nil {
			return Settlement{}, err
		}
		return Settlement{}, &BillConflictError{Bill: fresh}
	}
	sn, err := loadSnapshot(ctx, tx, visitID, v.billVersion)
	if err != nil {
		return Settlement{}, err
	}
	if *in.AmountSatang != sn.totals.TotalSatang {
		return Settlement{}, ErrAmountMismatch
	}
	// Member earning uses the benefit frozen at begin (LOY-004/005); the
	// profile lock precedes the capability and settlement rows.
	var prof profileRow
	var eligible, points *int64
	if sn.memberID != nil {
		if prof, err = lockProfile(ctx, tx, v.branchID, *sn.memberID); err != nil {
			return Settlement{}, err
		}
		e := EligibleSpend(sn.totals, sn.policy)
		pts := Points(e, *sn.satangPerPoint)
		eligible, points = &e, &pts
	}
	ref := receiptReference()
	var id string
	if err := tx.QueryRow(ctx, q("settlement_insert"), v.branchID, visitID, sn.id, ref, sn.totals.TotalSatang, in.Method,
		in.VerificationNote, in.ExternalReference, p.StaffID, sn.memberID, eligible, points, sn.loyaltyVersion).Scan(&id); err != nil {
		return Settlement{}, err
	}
	if sn.memberID != nil {
		if err := applyLedger(ctx, tx, v.branchID, *sn.memberID, id, "earn", prof, *points, *eligible, *sn.loyaltyVersion); err != nil {
			return Settlement{}, err
		}
	}
	if err := tx.QueryRow(ctx, q("visit_set_state"), visitID, "paid", false).Scan(&v.billVersion, &v.version); err != nil {
		return Settlement{}, err
	}
	v.state = "paid"
	if err := access.RevokeCapability(ctx, tx, access.KindVisit, visitID); err != nil && !errors.Is(err, access.ErrNoCapability) {
		return Settlement{}, err
	}
	if err := audit.Record(ctx, tx, audit.Event{BranchID: v.branchID, ActorStaffID: &p.StaffID, Action: "settlement.confirmed",
		ResourceType: "settlement", ResourceID: id, RequestID: requestID,
		Details: map[string]any{"visit_id": visitID, "amount_satang": sn.totals.TotalSatang, "method": in.Method,
			"receipt_reference": ref, "member_id": sn.memberID, "points_earned": points}}); err != nil {
		return Settlement{}, err
	}
	b, err := buildBill(ctx, tx, visitID, v)
	if err != nil {
		return Settlement{}, err
	}
	return Settlement{ID: id, VisitID: visitID, ReceiptReference: ref, AmountSatang: sn.totals.TotalSatang,
		Method: in.Method, PaidAt: b.Settlement.PaidAt, PointsEarned: points, Bill: b}, nil
}
