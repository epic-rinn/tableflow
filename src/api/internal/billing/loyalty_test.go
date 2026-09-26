package billing_test

import (
	"sync"
	"testing"

	"github.com/epic-rinn/tableflow/src/api/internal/testenv"
)

func (f *fixture) claim(g *testenv.Guest) testenv.Resp {
	bv := f.bill(f.e.Cashier).Num("bill_version")
	return g.Send(f.path("/member-claim"), "", map[string]any{"expected_version": bv})
}

// member returns a diner phone at this table signed in as a new member.
func (f *fixture) member(email string) (string, *testenv.Guest) {
	id, cookie := f.e.Member(email)
	return id, f.e.Diner(f.token).WithMember(cookie)
}

// paidMember seats nothing new: orders qty teas at the fixture visit, has
// the member claim it, serves, begins and confirms; returns the settlement.
func (f *fixture) paidMember(g *testenv.Guest, qty int) testenv.Resp {
	f.e.T.Helper()
	id, v := f.order(f.tea, qty)
	f.serve(id, v)
	if r := f.claim(g); r.Status != 200 {
		f.e.T.Fatalf("claim: %d %s", r.Status, r.Raw)
	}
	return f.settle()
}

// TestLoyaltyPolicyVersions: pilot defaults, bounds, manager only.
func TestLoyaltyPolicyVersions(t *testing.T) {
	f := setup(t)
	path := "/api/v1/branches/" + f.e.Branch + "/loyalty-policy"
	r := f.e.StaffGet(f.e.Cashier, path)
	if r.Status != 200 || r.Num("version") != 0 || r.Get("configured") != false || r.Num("satang_per_point") != 10000 ||
		r.Num("silver_threshold_satang") != 500000 || r.Num("gold_discount_bp") != 500 {
		t.Fatalf("defaults: %d %s", r.Status, r.Raw)
	}
	body := map[string]any{"expected_version": 0, "satang_per_point": 5000, "silver_threshold_satang": 300000,
		"silver_discount_bp": 200, "gold_threshold_satang": 1000000, "gold_discount_bp": 400}
	if r := f.e.StaffSend("PUT", f.e.Cashier, path, body); r.Status != 403 {
		t.Fatalf("cashier edit: %d", r.Status)
	}
	bad := map[string]any{"expected_version": 0, "satang_per_point": 5000, "silver_threshold_satang": 300000,
		"silver_discount_bp": 200, "gold_threshold_satang": 300000, "gold_discount_bp": 9000}
	if r := f.e.StaffSend("PUT", f.e.Manager, path, bad); r.Status != 422 || r.Get("error", "fields", "gold_threshold_satang") == nil ||
		r.Get("error", "fields", "gold_discount_bp") == nil {
		t.Fatalf("invalid: %d %s", r.Status, r.Raw)
	}
	if r := f.e.StaffSend("PUT", f.e.Manager, path, body); r.Status != 200 || r.Num("version") != 1 || r.Get("configured") != true {
		t.Fatalf("set: %d %s", r.Status, r.Raw)
	}
	if r := f.e.StaffSend("PUT", f.e.Manager, path, body); r.Code() != "VERSION_CONFLICT" {
		t.Fatalf("stale: %d", r.Status)
	}
}

// TestClaimNeedsBothSessions (LOY-A1): the shared QR alone cannot claim or
// read member data; other diners see only that the visit is claimed.
func TestClaimNeedsBothSessions(t *testing.T) {
	f := setup(t)
	f.order(f.tea, 1)
	bv := f.bill(f.e.Cashier).Num("bill_version")
	plain := f.e.Diner(f.token)
	if r := plain.Send(f.path("/member-claim"), "", map[string]any{"expected_version": bv}); r.Status != 401 {
		t.Fatalf("guest-only claim: %d", r.Status)
	}
	_, memberCookie := f.e.Member("solo@example.com")
	r := f.e.Do("POST", f.path("/member-claim"), "__Host-tf_member="+memberCookie, testenv.PWAOrigin, testenv.NewKey(), map[string]any{"expected_version": bv})
	if r.Status != 401 {
		t.Fatalf("member without table session: %d", r.Status)
	}
	_, otherToken := f.e.SeatWalkIn(f.e.Table("T9", 2), 1)
	elsewhere := f.e.Diner(otherToken).WithMember(memberCookie)
	if r := elsewhere.Send(f.path("/member-claim"), "", map[string]any{"expected_version": bv}); r.Status != 404 {
		t.Fatalf("claim from another table: %d", r.Status)
	}

	_, ann := f.member("ann@example.com")
	c := f.claim(ann)
	if c.Status != 200 || c.Get("claimed") != true || c.Get("mine") != true || c.Str("tier") != "base" {
		t.Fatalf("claim: %d %s", c.Status, c.Raw)
	}
	g := plain.Get(f.path("/bill"))
	if g.Get("member_claim", "claimed") != true || g.Get("member_claim", "tier") != nil || g.Get("member_claim", "masked_email") != nil {
		t.Fatalf("guest bill leaks member data: %s", g.Raw)
	}
	if s := f.bill(f.e.Cashier); s.Str("member_claim", "masked_email") != "a***@example.com" || s.Str("member_claim", "tier") != "base" {
		t.Fatalf("cashier bill: %s", s.Raw)
	}
	_, bob := f.member("bob@example.com")
	st := bob.Get(f.path("/member-claim"))
	if st.Status != 200 || st.Get("claimed") != true || st.Get("mine") != false || st.Get("tier") != nil || st.Get("discount_bp") != nil {
		t.Fatalf("other member's view: %s", st.Raw)
	}
	if r := plain.Get("/api/v1/members/me/loyalty"); r.Status != 401 {
		t.Fatalf("guest read member loyalty: %d", r.Status)
	}
}

// TestClaimConflictsAndDetach (LOY-002): no silent replacement; staff detach
// with a reason while open; settling claims are immutable.
func TestClaimConflictsAndDetach(t *testing.T) {
	f := setup(t)
	id, v := f.order(f.tea, 1)
	_, ann := f.member("ann@example.com")
	_, bob := f.member("bob@example.com")
	if r := f.claim(ann); r.Status != 200 {
		t.Fatalf("claim: %d %s", r.Status, r.Raw)
	}
	if r := f.claim(ann); r.Status != 200 || r.Get("mine") != true {
		t.Fatalf("re-claim by owner: %d %s", r.Status, r.Raw)
	}
	if r := f.claim(bob); r.Code() != "ALREADY_CLAIMED" {
		t.Fatalf("second member: %d %s", r.Status, r.Raw)
	}
	detach := func(cookie string, body map[string]any) testenv.Resp {
		return f.e.StaffSend("POST", cookie, f.path("/member-detach"), body)
	}
	bv := f.bill(f.e.Cashier).Num("bill_version")
	if r := detach(f.e.Host, map[string]any{"expected_version": bv, "reason": "wrong person"}); r.Status != 403 {
		t.Fatalf("host detach: %d", r.Status)
	}
	if r := detach(f.e.Cashier, map[string]any{"expected_version": bv}); r.Status != 422 {
		t.Fatalf("detach without reason: %d", r.Status)
	}
	if r := detach(f.e.Cashier, map[string]any{"expected_version": bv, "reason": "wrong person"}); r.Status != 200 || r.Get("member_claim") != nil {
		t.Fatalf("detach: %d %s", r.Status, r.Raw)
	}
	if n := f.e.Count("SELECT count(*) FROM audit_events WHERE action = 'visit.member_detached' AND reason = 'wrong person'"); n != 1 {
		t.Fatalf("detach audit rows = %d", n)
	}
	if r := f.claim(bob); r.Status != 200 || r.Get("mine") != true {
		t.Fatalf("replacement claim: %d %s", r.Status, r.Raw)
	}
	f.serve(id, v)
	s := f.begin(f.e.Cashier, f.bill(f.e.Cashier).Num("bill_version"))
	if r := detach(f.e.Cashier, map[string]any{"expected_version": s.Num("bill_version"), "reason": "late"}); r.Code() != "VISIT_STATE_CONFLICT" {
		t.Fatalf("detach while settling: %d %s", r.Status, r.Raw)
	}
	if r := f.claim(ann); r.Code() != "VISIT_STATE_CONFLICT" {
		t.Fatalf("claim while settling: %d %s", r.Status, r.Raw)
	}
}

// TestTierSnapshotAtBegin (LOY-A3): the bill that crosses a threshold uses
// the previous tier; the next visit gets the new tier; a tier change after
// begin does not alter the frozen bill. [LOY-003]
func TestTierSnapshotAtBegin(t *testing.T) {
	f := setup(t)
	memberID, ann := f.member("ann@example.com")
	f.e.Exec("INSERT INTO member_profiles (branch_id, member_id, qualifying_spend_satang) VALUES ($1, $2, 490000)", f.e.Branch, memberID)
	id, v := f.order(f.tea, 2) // 120.00 → crosses 5,000.00
	f.serve(id, v)
	if r := f.claim(ann); r.Status != 200 {
		t.Fatalf("claim: %s", r.Raw)
	}
	s := f.begin(f.e.Cashier, f.bill(f.e.Cashier).Num("bill_version"))
	if s.Num("discount_bp") != 0 || s.Num("total_satang") != 12000 {
		t.Fatalf("crossing bill discounted: %s", s.Raw)
	}
	// A tier change after begin never alters the frozen amount.
	f.e.Exec("UPDATE member_profiles SET tier = 'gold' WHERE member_id = $1", memberID)
	c := f.confirm(f.e.Cashier, "", s.Num("bill_version"), 12000)
	if c.Status != 201 || c.Num("points_earned") != 1 {
		t.Fatalf("confirm: %d %s", c.Status, c.Raw)
	}
	if tier := testenv.Scalar[string](f.e, "SELECT tier FROM member_profiles WHERE member_id = $1", memberID); tier != "silver" {
		t.Fatalf("tier after crossing = %s", tier)
	}

	f.visit, f.token = f.e.SeatWalkIn(f.e.Table("T2", 2), 1)
	ann2 := f.e.Diner(f.token).WithMember(f.e.MemberSession(memberID))
	id2, v2 := f.order(f.tea, 1)
	f.serve(id2, v2)
	if r := f.claim(ann2); r.Status != 200 || r.Str("tier") != "silver" || r.Num("discount_bp") != 300 {
		t.Fatalf("second visit claim: %s", r.Raw)
	}
	s2 := f.begin(f.e.Cashier, f.bill(f.e.Cashier).Num("bill_version"))
	// 60.00 − 3% (1.80) = 58.20.
	if s2.Num("discount_bp") != 300 || s2.Num("discount_satang") != 180 || s2.Num("total_satang") != 5820 {
		t.Fatalf("next visit discount: %s", s2.Raw)
	}
}

// TestConcurrentMemberSettlements (LOY-A2): two paid visits for one member
// both earn once; the balance equals the ledger sum. [LOY-005]
func TestConcurrentMemberSettlements(t *testing.T) {
	f := setup(t)
	memberID, _ := f.e.Member("ann@example.com")
	type visit struct {
		id, token string
		settle    testenv.Resp
	}
	visits := make([]*visit, 2)
	for i := range visits {
		vid, tok := f.e.SeatWalkIn(f.e.Table([]string{"A1", "A2"}[i], 2), 1)
		f.visit, f.token = vid, tok
		lid, lv := f.order(f.tea, 3+i) // 180.00 and 240.00
		f.serve(lid, lv)
		g := f.e.Diner(tok).WithMember(f.e.MemberSession(memberID))
		if r := f.claim(g); r.Status != 200 {
			t.Fatalf("claim %d: %s", i, r.Raw)
		}
		visits[i] = &visit{id: vid, token: tok, settle: f.begin(f.e.Cashier, f.bill(f.e.Cashier).Num("bill_version"))}
	}
	var wg sync.WaitGroup
	res := make([]testenv.Resp, 2)
	for i, v := range visits {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res[i] = f.e.Do("POST", "/api/v1/visits/"+v.id+"/settlement/confirm", testenv.StaffCookie(f.e.Cashier), testenv.AdminOrigin, testenv.NewKey(),
				map[string]any{"expected_version": v.settle.Num("bill_version"), "amount_satang": v.settle.Num("total_satang"), "method": "cash", "verification_note": "till"})
		}()
	}
	wg.Wait()
	for i, r := range res {
		if r.Status != 201 {
			t.Fatalf("confirm %d: %d %s", i, r.Status, r.Raw)
		}
	}
	if n := f.e.Count("SELECT count(*) FROM loyalty_ledger WHERE member_id = $1 AND kind = 'earn'", memberID); n != 2 {
		t.Fatalf("earn rows = %d", n)
	}
	if ok := testenv.Scalar[bool](f.e, `SELECT p.points_balance = (SELECT sum(points_delta) FROM loyalty_ledger WHERE member_id = $1)
		AND p.qualifying_spend_satang = (SELECT sum(qualifying_delta_satang) FROM loyalty_ledger WHERE member_id = $1)
		AND p.points_balance = 3 AND p.qualifying_spend_satang = 42000
		FROM member_profiles p WHERE p.member_id = $1`, memberID); !ok {
		t.Fatal("profile does not reconcile with the ledger (want 1 + 2 = 3 points, 420.00 qualifying)")
	}
}

// TestMemberRefundReversesOnce (LOY-A4, BIL-A6): refunds reverse the
// original award exactly once even after the rates change. [LOY-006]
func TestMemberRefundReversesOnce(t *testing.T) {
	f := setup(t)
	memberID, ann := f.member("ann@example.com")
	sid := f.paidMember(ann, 5).Str("id") // 300.00 → 3 points
	path := "/api/v1/branches/" + f.e.Branch + "/loyalty-policy"
	if r := f.e.StaffSend("PUT", f.e.Manager, path, map[string]any{"expected_version": 0, "satang_per_point": 100,
		"silver_threshold_satang": 1000, "silver_discount_bp": 100, "gold_threshold_satang": 2000, "gold_discount_bp": 200}); r.Status != 200 {
		t.Fatalf("policy: %s", r.Raw)
	}
	var wg sync.WaitGroup
	res := make([]testenv.Resp, 3)
	for i := range res {
		wg.Add(1)
		go func() { defer wg.Done(); res[i] = f.refund(f.e.Manager, "", sid) }()
	}
	wg.Wait()
	created := 0
	for _, r := range res {
		if r.Status == 201 {
			created++
		} else if r.Code() != "ALREADY_REFUNDED" {
			t.Fatalf("refund: %d %s", r.Status, r.Raw)
		}
	}
	if created != 1 {
		t.Fatalf("refunds created = %d", created)
	}
	rev := testenv.Scalar[int64](f.e, "SELECT points_delta FROM loyalty_ledger WHERE settlement_id = $1 AND kind = 'reversal'", sid)
	if rev != -3 || f.e.Count("SELECT count(*) FROM loyalty_ledger WHERE settlement_id = $1", sid) != 2 {
		t.Fatalf("reversal points = %d", rev)
	}
	if ok := testenv.Scalar[bool](f.e, "SELECT points_balance = 0 AND qualifying_spend_satang = 0 AND tier = 'base' FROM member_profiles WHERE member_id = $1", memberID); !ok {
		t.Fatal("profile not restored after refund")
	}
	if s := testenv.Scalar[string](f.e, "SELECT state FROM visits WHERE id = $1", f.visit); s != "paid" {
		t.Fatalf("visit state after refund = %s", s)
	}
	receipt := f.e.StaffGet(f.e.Cashier, "/api/v1/settlements/"+sid)
	if receipt.Num("member", "points_earned") != 3 || receipt.Str("refund", "reason") == "" {
		t.Fatalf("receipt: %s", receipt.Raw)
	}
}

// TestMemberLoyaltyHistory (LOY-007): own balance, tier progress and
// paginated history only.
func TestMemberLoyaltyHistory(t *testing.T) {
	f := setup(t)
	_, ann := f.member("ann@example.com")
	_, bob := f.member("bob@example.com") // before payment revokes the table QR
	sid := f.paidMember(ann, 2).Str("id") // 120.00 → 1 point
	if r := f.refund(f.e.Manager, "", sid); r.Status != 201 {
		t.Fatalf("refund: %s", r.Raw)
	}
	me := ann.Get("/api/v1/members/me/loyalty")
	if me.Status != 200 || me.Len("items") != 1 || me.Num("items", 0, "points") != 0 || me.Str("items", 0, "tier") != "base" ||
		me.Str("items", 0, "next_tier") != "silver" || me.Num("items", 0, "next_threshold_satang") != 500000 || me.Str("items", 0, "branch_name") != "Main" {
		t.Fatalf("loyalty: %d %s", me.Status, me.Raw)
	}
	if me.Header.Get("Cache-Control") != "private, no-store" {
		t.Fatalf("cache: %s", me.Header.Get("Cache-Control"))
	}
	page := ann.Get("/api/v1/members/me/loyalty/entries?limit=1")
	if page.Status != 200 || page.Len("items") != 1 || page.Str("items", 0, "kind") != "reversal" || page.Str("next_cursor") == "" {
		t.Fatalf("entries: %s", page.Raw)
	}
	next := ann.Get("/api/v1/members/me/loyalty/entries?limit=1&cursor=" + page.Str("next_cursor"))
	if next.Len("items") != 1 || next.Str("items", 0, "kind") != "earn" || next.Num("items", 0, "points") != 1 {
		t.Fatalf("page 2: %s", next.Raw)
	}
	if r := bob.Get("/api/v1/members/me/loyalty/entries"); r.Status != 200 || r.Len("items") != 0 {
		t.Fatalf("other member sees entries: %s", r.Raw)
	}
	// Payment revoked the table session, so a settled bill cannot be claimed.
	if r := f.claim(ann); r.Status != 401 {
		t.Fatalf("retrospective claim of a paid bill: %d %s", r.Status, r.Raw)
	}
}
