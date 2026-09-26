package billing

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/epic-rinn/tableflow/src/api/internal/access"
	"github.com/epic-rinn/tableflow/src/api/internal/identity"
	"github.com/epic-rinn/tableflow/src/api/internal/members"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/audit"
)

// Loyalty errors.
var (
	ErrAlreadyClaimed = errors.New("visit claimed by another member")
	ErrNotClaimed     = errors.New("visit has no member claim")
)

// --- policy ------------------------------------------------------------------

// LoyaltyPolicyView is the current loyalty policy with its creation time.
type LoyaltyPolicyView struct {
	LoyaltyPolicy
	UpdatedAt *time.Time `json:"updated_at"`
}

func scanLoyalty(row pgx.Row) (LoyaltyPolicy, bool, error) {
	var p LoyaltyPolicy
	err := row.Scan(&p.Version, &p.SatangPerPoint, &p.SilverThreshold, &p.SilverDiscountBP, &p.GoldThreshold, &p.GoldDiscountBP)
	if errors.Is(err, pgx.ErrNoRows) {
		return PilotLoyalty, false, nil
	}
	if err != nil {
		return LoyaltyPolicy{}, false, err
	}
	p.Configured = true
	return p, true, nil
}

func currentLoyalty(ctx context.Context, db querier, branchID string) (LoyaltyPolicy, error) {
	p, _, err := scanLoyalty(db.QueryRow(ctx, q("loyalty_policy_current"), branchID))
	return p, err
}

// LoyaltyPolicy returns the branch's current loyalty policy to its staff.
func (s *Service) LoyaltyPolicy(ctx context.Context, p identity.Principal, branchID string) (LoyaltyPolicy, error) {
	if p.BranchID != branchID {
		return LoyaltyPolicy{}, ErrNotFound
	}
	return currentLoyalty(ctx, s.pool, branchID)
}

// LoyaltyPolicyIn is a manager's loyalty-policy edit.
type LoyaltyPolicyIn struct {
	ExpectedVersion  *int   `json:"expected_version"`
	SatangPerPoint   *int64 `json:"satang_per_point"`
	SilverThreshold  *int64 `json:"silver_threshold_satang"`
	SilverDiscountBP *int   `json:"silver_discount_bp"`
	GoldThreshold    *int64 `json:"gold_threshold_satang"`
	GoldDiscountBP   *int   `json:"gold_discount_bp"`
}

func (in LoyaltyPolicyIn) validate() error {
	f := map[string]string{}
	if in.ExpectedVersion == nil || *in.ExpectedVersion < 0 {
		f["expected_version"] = "is required"
	}
	if in.SatangPerPoint == nil || *in.SatangPerPoint < 100 || *in.SatangPerPoint > 10000000 {
		f["satang_per_point"] = "must be ฿1–฿100,000 per point"
	}
	if in.SilverThreshold == nil || *in.SilverThreshold <= 0 {
		f["silver_threshold_satang"] = "must be positive"
	}
	if in.GoldThreshold == nil || *in.GoldThreshold <= 0 {
		f["gold_threshold_satang"] = "must be positive"
	} else if in.SilverThreshold != nil && *in.GoldThreshold <= *in.SilverThreshold {
		f["gold_threshold_satang"] = "must be above the Silver threshold"
	}
	for k, v := range map[string]*int{"silver_discount_bp": in.SilverDiscountBP, "gold_discount_bp": in.GoldDiscountBP} {
		if v == nil || *v < 0 || *v > 5000 {
			f[k] = "must be 0–5000 basis points"
		}
	}
	if len(f) > 0 {
		return &ValidationError{Fields: f}
	}
	return nil
}

// SetLoyaltyPolicy appends a new version (manager). No branch lock: the
// (branch, version) key arbitrates concurrent edits (see charge policy).
func (s *Service) SetLoyaltyPolicy(ctx context.Context, tx pgx.Tx, p identity.Principal, branchID string, in LoyaltyPolicyIn, requestID string) (LoyaltyPolicy, error) {
	if err := in.validate(); err != nil {
		return LoyaltyPolicy{}, err
	}
	p, err := s.revalidate(ctx, tx, p, []string{identity.RoleManager})
	if err != nil {
		return LoyaltyPolicy{}, err
	}
	if p.BranchID != branchID {
		return LoyaltyPolicy{}, ErrNotFound
	}
	cur, err := currentLoyalty(ctx, tx, branchID)
	if err != nil {
		return LoyaltyPolicy{}, err
	}
	if cur.Version != *in.ExpectedVersion {
		return LoyaltyPolicy{}, ErrVersionConflict
	}
	next := LoyaltyPolicy{Version: cur.Version + 1, SatangPerPoint: *in.SatangPerPoint, SilverThreshold: *in.SilverThreshold,
		SilverDiscountBP: *in.SilverDiscountBP, GoldThreshold: *in.GoldThreshold, GoldDiscountBP: *in.GoldDiscountBP, Configured: true}
	if _, err := tx.Exec(ctx, q("loyalty_policy_insert"), branchID, next.Version, next.SatangPerPoint, next.SilverThreshold,
		next.SilverDiscountBP, next.GoldThreshold, next.GoldDiscountBP, p.StaffID); err != nil {
		if pg := (*pgconn.PgError)(nil); errors.As(err, &pg) && pg.Code == "23505" {
			return LoyaltyPolicy{}, ErrVersionConflict
		}
		return LoyaltyPolicy{}, err
	}
	err = audit.Record(ctx, tx, audit.Event{BranchID: branchID, ActorStaffID: &p.StaffID, Action: "loyalty_policy.updated",
		ResourceType: "loyalty_policy", ResourceID: branchID, RequestID: requestID,
		Details: map[string]any{"version": next.Version, "satang_per_point": next.SatangPerPoint, "silver_threshold": next.SilverThreshold,
			"silver_discount_bp": next.SilverDiscountBP, "gold_threshold": next.GoldThreshold, "gold_discount_bp": next.GoldDiscountBP}})
	return next, err
}

// --- profiles ----------------------------------------------------------------

type profileRow struct {
	id                string
	points, qualifies int64
	tier              string
}

func scanProfile(row pgx.Row) (profileRow, error) {
	var p profileRow
	err := row.Scan(&p.id, &p.points, &p.qualifies, &p.tier)
	return p, err
}

// memberTier reads a profile's tier without locking (bill preview).
func memberTier(ctx context.Context, db querier, branchID, memberID string) (string, error) {
	p, err := scanProfile(db.QueryRow(ctx, q("profile_get"), branchID, memberID))
	if errors.Is(err, pgx.ErrNoRows) {
		return TierBase, nil
	}
	return p.tier, err
}

// lockProfile locks (creating if needed) a member's branch profile. Callers
// hold the visit lock first (lock order: visits → member profiles).
func lockProfile(ctx context.Context, tx pgx.Tx, branchID, memberID string) (profileRow, error) {
	if _, err := tx.Exec(ctx, q("profile_ensure"), branchID, memberID); err != nil {
		return profileRow{}, err
	}
	return scanProfile(tx.QueryRow(ctx, q("profile_lock"), branchID, memberID))
}

// applyLedger appends one ledger entry and moves the profile totals by the
// same amounts, recalculating the tier with the current thresholds (LOY-005/006).
func applyLedger(ctx context.Context, tx pgx.Tx, branchID, memberID, settlementID, kind string, prof profileRow, points, qualifying int64, policyVersion int) error {
	if _, err := tx.Exec(ctx, q("ledger_insert"), branchID, memberID, settlementID, kind, points, qualifying, policyVersion); err != nil {
		return err
	}
	lp, err := currentLoyalty(ctx, tx, branchID)
	if err != nil {
		return err
	}
	var balance, total int64
	return tx.QueryRow(ctx, q("profile_apply"), prof.id, points, qualifying, lp.Tier(prof.qualifies+qualifying)).Scan(&balance, &total)
}

func maskEmail(email string) string {
	at := strings.LastIndexByte(email, '@')
	if at <= 0 {
		return "***"
	}
	return email[:1] + "***" + email[at:]
}

// --- claim -------------------------------------------------------------------

// ClaimStatus is the claim as seen by a signed-in diner at the table.
type ClaimStatus struct {
	Claimed     bool    `json:"claimed"`
	Mine        bool    `json:"mine"`
	Tier        *string `json:"tier,omitempty"`
	DiscountBP  *int    `json:"discount_bp,omitempty"`
	BillVersion int     `json:"bill_version"`
	VisitState  string  `json:"visit_state"`
}

func claimStatus(v visitRow, memberID string, tier string, lp LoyaltyPolicy) ClaimStatus {
	cs := ClaimStatus{Claimed: v.memberID != nil, BillVersion: v.billVersion, VisitState: v.state}
	if v.memberID != nil && *v.memberID == memberID {
		d := lp.DiscountBP(tier)
		cs.Mine, cs.Tier, cs.DiscountBP = true, &tier, &d
	}
	return cs
}

func guestOwnsVisit(g access.Guest, visitID string) bool {
	return g.Kind == access.KindVisit && g.ResourceID == visitID
}

// Claim links the visit to the member (LOY-001/002). It needs the member
// session and this visit's guest session; the shared QR alone cannot claim.
func (s *Service) Claim(ctx context.Context, tx pgx.Tx, m members.Member, g access.Guest, visitID string, expected int, requestID string) (ClaimStatus, error) {
	if err := members.Revalidate(ctx, tx, m); err != nil { // principal rows first
		return ClaimStatus{}, err
	}
	v, err := scanVisit(tx.QueryRow(ctx, q("visit_lock"), visitID))
	if err != nil {
		return ClaimStatus{}, err
	}
	g, err = access.RevalidateGuest(ctx, tx, g)
	if err != nil {
		return ClaimStatus{}, err
	}
	if !guestOwnsVisit(g, visitID) {
		return ClaimStatus{}, ErrNotFound
	}
	if v.state != "open" {
		return ClaimStatus{}, ErrVisitState // settling/paid claims are immutable
	}
	if v.memberID != nil && *v.memberID != m.ID {
		return ClaimStatus{}, ErrAlreadyClaimed
	}
	prof, err := lockProfile(ctx, tx, v.branchID, m.ID)
	if err != nil {
		return ClaimStatus{}, err
	}
	lp, err := currentLoyalty(ctx, tx, v.branchID)
	if err != nil {
		return ClaimStatus{}, err
	}
	if v.memberID != nil { // already this member's: idempotent
		return claimStatus(v, m.ID, prof.tier, lp), nil
	}
	if v.billVersion != expected {
		return ClaimStatus{}, ErrVersionConflict
	}
	if err := tx.QueryRow(ctx, q("visit_set_member"), visitID, m.ID).Scan(&v.billVersion); err != nil {
		return ClaimStatus{}, err
	}
	v.memberID = &m.ID
	if err := audit.Record(ctx, tx, audit.Event{BranchID: v.branchID, Action: "visit.member_claimed", ResourceType: "visit",
		ResourceID: visitID, RequestID: requestID, Details: map[string]any{"member_id": m.ID}}); err != nil {
		return ClaimStatus{}, err
	}
	return claimStatus(v, m.ID, prof.tier, lp), nil
}

// ClaimStatus reports the claim to a signed-in member at this table; other
// members only learn that the visit is claimed.
func (s *Service) ClaimStatus(ctx context.Context, m members.Member, g access.Guest, visitID string) (ClaimStatus, error) {
	if !guestOwnsVisit(g, visitID) {
		return ClaimStatus{}, ErrNotFound
	}
	v, err := scanVisit(s.pool.QueryRow(ctx, q("visit_get"), visitID))
	if err != nil {
		return ClaimStatus{}, err
	}
	tier, err := memberTier(ctx, s.pool, v.branchID, m.ID)
	if err != nil {
		return ClaimStatus{}, err
	}
	lp, err := currentLoyalty(ctx, s.pool, v.branchID)
	if err != nil {
		return ClaimStatus{}, err
	}
	return claimStatus(v, m.ID, tier, lp), nil
}

// Detach removes a claim while the visit is open, with an audited reason
// (LOY-002). The replacement member must claim again themselves.
func (s *Service) Detach(ctx context.Context, tx pgx.Tx, p identity.Principal, visitID string, expected int, reason, requestID string) (Bill, error) {
	r, ok := validText(reason, 500)
	if !ok {
		return Bill{}, &ValidationError{Fields: map[string]string{"reason": "must be 1–500 characters"}}
	}
	p, v, err := s.lockVisit(ctx, tx, p, visitID)
	if err != nil {
		return Bill{}, err
	}
	if err := checkState(ctx, tx, visitID, v, "open"); err != nil {
		return Bill{}, err
	}
	if v.memberID == nil {
		return Bill{}, ErrNotClaimed
	}
	if v.billVersion != expected {
		fresh, err := buildBill(ctx, tx, visitID, v)
		if err != nil {
			return Bill{}, err
		}
		return Bill{}, &BillConflictError{Bill: fresh}
	}
	former := *v.memberID
	if err := tx.QueryRow(ctx, q("visit_set_member"), visitID, nil).Scan(&v.billVersion); err != nil {
		return Bill{}, err
	}
	v.memberID = nil
	if err := audit.Record(ctx, tx, audit.Event{BranchID: v.branchID, ActorStaffID: &p.StaffID, Action: "visit.member_detached",
		ResourceType: "visit", ResourceID: visitID, Reason: &r, RequestID: requestID,
		Details: map[string]any{"member_id": former}}); err != nil {
		return Bill{}, err
	}
	return buildBill(ctx, tx, visitID, v)
}

// --- member history (LOY-007) ------------------------------------------------

// MemberLoyalty is one branch profile as the member sees it.
type MemberLoyalty struct {
	BranchID            string  `json:"branch_id"`
	BranchName          string  `json:"branch_name"`
	Points              int64   `json:"points"`
	QualifyingSpend     int64   `json:"qualifying_spend_satang"`
	Tier                string  `json:"tier"`
	DiscountBP          int     `json:"discount_bp"`
	NextTier            *string `json:"next_tier"`
	NextThresholdSatang *int64  `json:"next_threshold_satang"`
	SatangPerPoint      int64   `json:"satang_per_point"`
	PolicyConfigured    bool    `json:"policy_configured"`
}

// MemberLoyalty lists the member's own profiles.
func (s *Service) MemberLoyalty(ctx context.Context, m members.Member) ([]MemberLoyalty, error) {
	rows, err := s.pool.Query(ctx, q("member_loyalty"), m.ID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (MemberLoyalty, error) {
		var ml MemberLoyalty
		var version *int
		var spp, st, gt *int64
		var sd, gd *int
		if err := r.Scan(&ml.BranchID, &ml.BranchName, &ml.Points, &ml.QualifyingSpend, &ml.Tier, &version, &spp, &st, &sd, &gt, &gd); err != nil {
			return ml, err
		}
		lp := PilotLoyalty
		if version != nil {
			lp = LoyaltyPolicy{Version: *version, SatangPerPoint: *spp, SilverThreshold: *st, SilverDiscountBP: *sd, GoldThreshold: *gt, GoldDiscountBP: *gd, Configured: true}
		}
		ml.DiscountBP, ml.SatangPerPoint, ml.PolicyConfigured = lp.DiscountBP(ml.Tier), lp.SatangPerPoint, lp.Configured
		if next, th := lp.NextTier(ml.QualifyingSpend); next != "" {
			ml.NextTier, ml.NextThresholdSatang = &next, &th
		}
		return ml, nil
	})
}

// LedgerEntry is one row of the member's history.
type LedgerEntry struct {
	ID               string    `json:"id"`
	Kind             string    `json:"kind"`
	Points           int64     `json:"points"`
	QualifyingSatang int64     `json:"qualifying_satang"`
	ReceiptReference string    `json:"receipt_reference"`
	BranchName       string    `json:"branch_name"`
	CreatedAt        time.Time `json:"created_at"`
}

// LedgerPage is a keyset page of history, newest first.
type LedgerPage struct {
	Items      []LedgerEntry `json:"items"`
	NextCursor *string       `json:"next_cursor"`
}

// MemberEntries pages the member's own ledger.
func (s *Service) MemberEntries(ctx context.Context, m members.Member, after string, limit int) (LedgerPage, error) {
	if limit <= 0 {
		limit = pageDefault
	}
	limit = min(limit, pageMax)
	c := cursor{T: time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC), ID: "ffffffff-ffff-ffff-ffff-ffffffffffff", B: m.ID}
	if after != "" {
		b, err := base64.RawURLEncoding.DecodeString(after)
		if err != nil || json.Unmarshal(b, &c) != nil || c.B != m.ID {
			return LedgerPage{}, &ValidationError{Fields: map[string]string{"cursor": "invalid cursor"}}
		}
	}
	rows, err := s.pool.Query(ctx, q("member_entries"), m.ID, c.T, c.ID, limit+1)
	if err != nil {
		return LedgerPage{}, err
	}
	items, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (LedgerEntry, error) {
		var e LedgerEntry
		return e, r.Scan(&e.ID, &e.Kind, &e.Points, &e.QualifyingSatang, &e.CreatedAt, &e.ReceiptReference, &e.BranchName)
	})
	if err != nil {
		return LedgerPage{}, err
	}
	page := LedgerPage{Items: items}
	if len(items) > limit {
		page.Items = items[:limit]
		last := page.Items[limit-1]
		b, _ := json.Marshal(cursor{T: last.CreatedAt, ID: last.ID, B: m.ID})
		next := base64.RawURLEncoding.EncodeToString(b)
		page.NextCursor = &next
	}
	return page, nil
}
