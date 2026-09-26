package billing_test

import (
	"sync"
	"testing"

	"github.com/epic-rinn/tableflow/src/api/internal/testenv"
)

// fixture is a seated visit with a two-item menu.
type fixture struct {
	e        *testenv.Env
	visit    string
	token    string
	table    string
	revision int
	tea      string // 60.00
	rice     string // 45.50
}

func setup(t *testing.T) *fixture {
	e := testenv.New(t)
	r := e.Do("PUT", "/api/v1/branches/"+e.Branch+"/menu", testenv.StaffCookie(e.Manager), testenv.AdminOrigin, "", map[string]any{
		"expected_revision": 1,
		"categories": []map[string]any{{"name_th": "อาหาร", "name_en": "Food", "items": []map[string]any{
			{"name_th": "ชาไทย", "name_en": "Thai Tea", "price_satang": 6000, "option_groups": []map[string]any{}},
			{"name_th": "ข้าว", "name_en": "Rice", "price_satang": 4550, "option_groups": []map[string]any{}},
		}}},
	})
	if r.Status != 200 {
		t.Fatalf("menu: %d %s", r.Status, r.Raw)
	}
	f := &fixture{e: e, revision: int(r.Num("revision"))}
	f.tea = r.Str("categories", 0, "items", 0, "id")
	f.rice = r.Str("categories", 0, "items", 1, "id")
	f.table = e.Table("T1", 4)
	f.visit, f.token = e.SeatWalkIn(f.table, 2)
	return f
}

func (f *fixture) path(suffix string) string { return "/api/v1/visits/" + f.visit + suffix }

// order places one line and returns its ID and version.
func (f *fixture) order(item string, qty int) (string, int) {
	f.e.T.Helper()
	r := f.e.Diner(f.token).Send(f.path("/orders"), "", map[string]any{"menu_revision": f.revision,
		"lines": []map[string]any{{"item_id": item, "quantity": qty, "option_ids": []string{}}}})
	if r.Status != 201 {
		f.e.T.Fatalf("order: %d %s", r.Status, r.Raw)
	}
	return r.Str("lines", 0, "id"), int(r.Num("lines", 0, "version"))
}

// serve walks a line to served.
func (f *fixture) serve(id string, v int) {
	f.e.T.Helper()
	for _, to := range []string{"accepted", "preparing", "ready", "served"} {
		r := f.e.StaffSend("POST", f.e.Kitchen, "/api/v1/order-lines/"+id+"/transition", map[string]any{"expected_version": v, "to_state": to})
		if r.Status != 200 {
			f.e.T.Fatalf("%s: %d %s", to, r.Status, r.Raw)
		}
		v = int(r.Num("version"))
	}
}

func (f *fixture) setPolicy(mode string, tax, service int) {
	f.e.T.Helper()
	cur := f.e.StaffGet(f.e.Manager, "/api/v1/branches/"+f.e.Branch+"/charge-policy")
	r := f.e.StaffSend("PUT", f.e.Manager, "/api/v1/branches/"+f.e.Branch+"/charge-policy",
		map[string]any{"expected_version": cur.Num("version"), "tax_mode": mode, "tax_bp": tax, "service_bp": service})
	if r.Status != 200 {
		f.e.T.Fatalf("policy: %d %s", r.Status, r.Raw)
	}
}

func (f *fixture) bill(cookie string) testenv.Resp {
	return f.e.StaffGet(cookie, f.path("/bill"))
}

func (f *fixture) begin(cookie string, version int64) testenv.Resp {
	return f.e.StaffSend("POST", cookie, f.path("/settlement/begin"), map[string]any{"expected_version": version})
}

func (f *fixture) confirm(cookie, key string, version, amount int64) testenv.Resp {
	if key == "" {
		key = testenv.NewKey()
	}
	return f.e.Do("POST", f.path("/settlement/confirm"), testenv.StaffCookie(cookie), testenv.AdminOrigin, key, map[string]any{
		"expected_version": version, "amount_satang": amount, "method": "cash", "verification_note": "counted at till"})
}

// settle serves every line, begins and confirms; returns the settlement.
func (f *fixture) settle() testenv.Resp {
	f.e.T.Helper()
	b := f.bill(f.e.Cashier)
	s := f.begin(f.e.Cashier, b.Num("bill_version"))
	if s.Status != 200 {
		f.e.T.Fatalf("begin: %d %s", s.Status, s.Raw)
	}
	c := f.confirm(f.e.Cashier, "", s.Num("bill_version"), s.Num("total_satang"))
	if c.Status != 201 {
		f.e.T.Fatalf("confirm: %d %s", c.Status, c.Raw)
	}
	return c
}

// TestChargePolicyVersions: append-only versions, bounds, manager only.
func TestChargePolicyVersions(t *testing.T) {
	f := setup(t)
	path := "/api/v1/branches/" + f.e.Branch + "/charge-policy"
	r := f.e.StaffGet(f.e.Cashier, path)
	if r.Status != 200 || r.Num("version") != 0 || r.Get("configured") != false || r.Str("tax_mode") != "exclusive" {
		t.Fatalf("unconfigured: %d %s", r.Status, r.Raw)
	}
	body := map[string]any{"expected_version": 0, "tax_mode": "exclusive", "tax_bp": 700, "service_bp": 1000}
	if r := f.e.StaffSend("PUT", f.e.Cashier, path, body); r.Status != 403 {
		t.Fatalf("cashier edit: %d", r.Status)
	}
	for _, bad := range []map[string]any{
		{"expected_version": 0, "tax_mode": "vat", "tax_bp": 700, "service_bp": 0},
		{"expected_version": 0, "tax_mode": "exclusive", "tax_bp": 10001, "service_bp": 0},
		{"expected_version": 0, "tax_mode": "exclusive", "tax_bp": 0, "service_bp": -1},
		{"tax_mode": "exclusive", "tax_bp": 0, "service_bp": 0},
	} {
		if r := f.e.StaffSend("PUT", f.e.Manager, path, bad); r.Status != 422 {
			t.Fatalf("invalid %v: %d", bad, r.Status)
		}
	}
	if r := f.e.StaffSend("PUT", f.e.Manager, path, body); r.Status != 200 || r.Num("version") != 1 || r.Get("configured") != true {
		t.Fatalf("set: %d %s", r.Status, r.Raw)
	}
	if r := f.e.StaffSend("PUT", f.e.Manager, path, body); r.Code() != "VERSION_CONFLICT" {
		t.Fatalf("stale version: %d %s", r.Status, r.Raw)
	}
	if n := f.e.Count("SELECT count(*) FROM audit_events WHERE action = 'charge_policy.updated'"); n != 1 {
		t.Fatalf("audit rows = %d", n)
	}
	// Concurrent edits of the same version: one wins, the other conflicts.
	next := map[string]any{"expected_version": 1, "tax_mode": "inclusive", "tax_bp": 700, "service_bp": 0}
	var wg sync.WaitGroup
	res := make([]testenv.Resp, 4)
	for i := range res {
		wg.Add(1)
		go func() { defer wg.Done(); res[i] = f.e.StaffSend("PUT", f.e.Manager, path, next) }()
	}
	wg.Wait()
	won := 0
	for _, r := range res {
		switch {
		case r.Status == 200:
			won++
		case r.Code() != "VERSION_CONFLICT":
			t.Fatalf("concurrent edit: %d %s", r.Status, r.Raw)
		}
	}
	if won != 1 {
		t.Fatalf("concurrent policy edits won = %d", won)
	}
}

// TestBeginRequiresCharges: a visit without chargeable lines is closed as
// empty (SEA-004), not settled at zero.
func TestBeginRequiresCharges(t *testing.T) {
	f := setup(t)
	id, v := f.order(f.tea, 1)
	if r := f.e.StaffSend("POST", f.e.Kitchen, "/api/v1/order-lines/"+id+"/transition",
		map[string]any{"expected_version": v, "to_state": "rejected", "reason": "none left"}); r.Status != 200 {
		t.Fatalf("reject: %d", r.Status)
	}
	if r := f.begin(f.e.Cashier, f.bill(f.e.Cashier).Num("bill_version")); r.Code() != "NOTHING_TO_SETTLE" {
		t.Fatalf("empty begin: %d %s", r.Status, r.Raw)
	}
}

// TestBillUsesSnapshotsAndChargeableLines (BIL-A2 integration): rejected
// lines are excluded, later menu price changes do not alter charges, and
// clients cannot supply totals.
func TestBillUsesSnapshotsAndChargeableLines(t *testing.T) {
	f := setup(t)
	f.setPolicy("exclusive", 700, 1000)
	f.order(f.tea, 2)  // 12000
	f.order(f.rice, 1) // 4550
	rej, rv := f.order(f.tea, 5)
	if r := f.e.StaffSend("POST", f.e.Kitchen, "/api/v1/order-lines/"+rej+"/transition",
		map[string]any{"expected_version": rv, "to_state": "rejected", "reason": "out of tea"}); r.Status != 200 {
		t.Fatalf("reject: %d", r.Status)
	}
	// Reprice the menu after ordering: snapshots keep the old price.
	f.e.Exec("UPDATE menu_items SET price_satang = 99900 WHERE id = $1", f.tea)
	b := f.bill(f.e.Cashier)
	// gross 16550; service 1655; base 18205 × 7% = 1274.35 → 1274; total 19479.
	if b.Status != 200 || b.Num("gross_satang") != 16550 || b.Num("service_satang") != 1655 ||
		b.Num("tax_satang") != 1274 || b.Num("total_satang") != 19479 || b.Len("lines") != 2 || b.Num("unresolved_lines") != 2 {
		t.Fatalf("bill: %d %s", b.Status, b.Raw)
	}
	if b.Header.Get("Cache-Control") != "private, no-store" || b.Get("frozen") != false {
		t.Fatalf("headers/frozen: %s", b.Raw)
	}
	f.setPolicy("inclusive", 700, 0)
	// Inclusive: total 16550; tax 16550 × 700/10700 = 1082.71 → 1083.
	b = f.bill(f.e.Cashier)
	if b.Num("total_satang") != 16550 || b.Num("tax_satang") != 1083 || b.Num("policy", "version") != 2 {
		t.Fatalf("inclusive bill: %s", b.Raw)
	}
}

// TestBillAccess: the visit's guest and cashier/manager read; others do not.
func TestBillAccess(t *testing.T) {
	f := setup(t)
	f.order(f.tea, 1)
	if r := f.e.Diner(f.token).Get(f.path("/bill")); r.Status != 200 || r.Num("gross_satang") != 6000 {
		t.Fatalf("guest bill: %d %s", r.Status, r.Raw)
	}
	for cookie, want := range map[string]int{f.e.Cashier: 200, f.e.Manager: 200, f.e.Kitchen: 403, f.e.Host: 403} {
		if r := f.bill(cookie); r.Status != want {
			t.Fatalf("staff bill: %d want %d", r.Status, want)
		}
	}
	other, otherToken := f.e.SeatWalkIn(f.e.Table("T2", 2), 1)
	if r := f.e.Diner(otherToken).Get(f.path("/bill")); r.Status != 404 {
		t.Fatalf("other visit's guest read this bill: %d", r.Status)
	}
	_ = other
	resolve := func(cookie, token string) testenv.Resp {
		return f.e.StaffSend("POST", cookie, "/api/v1/bills/resolve", map[string]any{"dining_token": token})
	}
	if r := resolve(f.e.Cashier, f.token); r.Status != 200 || r.Str("visit_id") != f.visit {
		t.Fatalf("resolve: %d %s", r.Status, r.Raw)
	}
	if r := resolve(f.e.Cashier, "not-a-token"); r.Status != 404 {
		t.Fatalf("bad token: %d", r.Status)
	}
	if r := resolve(f.e.Kitchen, f.token); r.Status != 403 {
		t.Fatalf("kitchen resolve: %d", r.Status)
	}
}

// TestBeginRequiresResolvedLines (BIL-002).
func TestBeginRequiresResolvedLines(t *testing.T) {
	f := setup(t)
	id, v := f.order(f.tea, 1)
	b := f.bill(f.e.Cashier)
	r := f.begin(f.e.Cashier, b.Num("bill_version"))
	if r.Code() != "UNRESOLVED_LINES" || r.Len("lines") != 1 || r.Str("lines", 0, "id") != id {
		t.Fatalf("unresolved: %d %s", r.Status, r.Raw)
	}
	f.serve(id, v)
	if r := f.begin(f.e.Cashier, b.Num("bill_version")); r.Status != 200 || r.Str("visit_state") != "settling" || r.Get("frozen") != true {
		t.Fatalf("begin: %d %s", r.Status, r.Raw)
	}
	if r := f.begin(f.e.Kitchen, b.Num("bill_version")); r.Status != 403 {
		t.Fatalf("kitchen begin: %d", r.Status)
	}
}

// TestBeginRejectsStaleBill (BIL-A1): a financial change after display
// yields a conflict carrying the fresh bill.
func TestBeginRejectsStaleBill(t *testing.T) {
	f := setup(t)
	id, v := f.order(f.tea, 1)
	f.serve(id, v)
	shown := f.bill(f.e.Cashier)
	id2, v2 := f.order(f.rice, 1)
	f.serve(id2, v2)
	r := f.begin(f.e.Cashier, shown.Num("bill_version"))
	if r.Code() != "BILL_VERSION_CONFLICT" || r.Num("bill", "gross_satang") != 10550 {
		t.Fatalf("stale begin: %d %s", r.Status, r.Raw)
	}
	if r := f.begin(f.e.Cashier, r.Num("bill", "bill_version")); r.Status != 200 {
		t.Fatalf("fresh begin: %d %s", r.Status, r.Raw)
	}
}

// TestSettlementVersusOrderSubmission (ORD-A4): once settling, orders and
// financial edits are refused; policy changes do not alter the snapshot.
func TestSettlementVersusOrderSubmission(t *testing.T) {
	f := setup(t)
	id, v := f.order(f.tea, 1)
	f.serve(id, v)
	s := f.begin(f.e.Cashier, f.bill(f.e.Cashier).Num("bill_version"))
	r := f.e.Diner(f.token).Send(f.path("/orders"), "", map[string]any{"menu_revision": f.revision,
		"lines": []map[string]any{{"item_id": f.tea, "quantity": 1, "option_ids": []string{}}}})
	if r.Code() != "VISIT_STATE_CONFLICT" {
		t.Fatalf("order while settling: %d %s", r.Status, r.Raw)
	}
	f.setPolicy("exclusive", 5000, 5000)
	if b := f.bill(f.e.Cashier); b.Num("total_satang") != s.Num("total_satang") || b.Num("policy", "version") != 0 {
		t.Fatalf("frozen bill changed: %s", b.Raw)
	}
	// Concurrent begin and order: exactly one of them wins the visit lock.
	f2 := setup(t)
	f2.serve(f2.order(f2.rice, 1))
	var wg sync.WaitGroup
	var begin, order testenv.Resp
	bv := f2.bill(f2.e.Cashier).Num("bill_version")
	g := f2.e.Diner(f2.token)
	wg.Add(2)
	go func() { defer wg.Done(); begin = f2.begin(f2.e.Cashier, bv) }()
	go func() {
		defer wg.Done()
		order = g.Send(f2.path("/orders"), "", map[string]any{"menu_revision": f2.revision,
			"lines": []map[string]any{{"item_id": f2.tea, "quantity": 1, "option_ids": []string{}}}})
	}()
	wg.Wait()
	switch {
	case begin.Status == 200 && order.Code() == "VISIT_STATE_CONFLICT":
	case order.Status == 201 && begin.Code() == "BILL_VERSION_CONFLICT":
	default:
		t.Fatalf("race outcome: begin %d %s / order %d %s", begin.Status, begin.Raw, order.Status, order.Raw)
	}
}

// TestReopenInvalidatesConfirmation (BIL-A5).
func TestReopenInvalidatesConfirmation(t *testing.T) {
	f := setup(t)
	id, v := f.order(f.tea, 1)
	f.serve(id, v)
	s := f.begin(f.e.Cashier, f.bill(f.e.Cashier).Num("bill_version"))
	old := s.Num("bill_version")
	reopen := func(body map[string]any) testenv.Resp {
		return f.e.StaffSend("POST", f.e.Cashier, f.path("/settlement/reopen"), body)
	}
	if r := reopen(map[string]any{"expected_version": old}); r.Status != 422 {
		t.Fatalf("reopen without reason: %d", r.Status)
	}
	r := reopen(map[string]any{"expected_version": old, "reason": "guest wants dessert"})
	if r.Status != 200 || r.Str("visit_state") != "open" || r.Num("bill_version") <= old {
		t.Fatalf("reopen: %d %s", r.Status, r.Raw)
	}
	id2, v2 := f.order(f.rice, 1)
	f.serve(id2, v2)
	if c := f.confirm(f.e.Cashier, "", old, s.Num("total_satang")); c.Code() != "VISIT_STATE_CONFLICT" {
		t.Fatalf("confirm while open: %d %s", c.Status, c.Raw)
	}
	s2 := f.begin(f.e.Cashier, f.bill(f.e.Cashier).Num("bill_version"))
	if c := f.confirm(f.e.Cashier, "", old, s.Num("total_satang")); c.Code() != "BILL_VERSION_CONFLICT" {
		t.Fatalf("old version confirm: %d %s", c.Status, c.Raw)
	}
	if c := f.confirm(f.e.Cashier, "", s2.Num("bill_version"), s.Num("total_satang")); c.Code() != "AMOUNT_MISMATCH" {
		t.Fatalf("old amount: %d %s", c.Status, c.Raw)
	}
	if c := f.confirm(f.e.Cashier, "", s2.Num("bill_version"), s2.Num("total_satang")); c.Status != 201 {
		t.Fatalf("confirm: %d %s", c.Status, c.Raw)
	}
	if n := f.e.Count("SELECT count(*) FROM audit_events WHERE action = 'settlement.reopened' AND reason = 'guest wants dessert'"); n != 1 {
		t.Fatalf("reopen audit rows = %d", n)
	}
}

// TestConcurrentConfirmOneSettlement (BIL-A3): two cashiers and a
// response-loss retry leave one settlement; the loser sees ALREADY_PAID.
func TestConcurrentConfirmOneSettlement(t *testing.T) {
	f := setup(t)
	id, v := f.order(f.tea, 1)
	f.serve(id, v)
	s := f.begin(f.e.Cashier, f.bill(f.e.Cashier).Num("bill_version"))
	second := f.e.StaffMember("cashier2@example.com", "cashier")
	key := testenv.NewKey()
	var wg sync.WaitGroup
	res := make([]testenv.Resp, 2)
	for i, c := range []string{f.e.Cashier, second} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			k := key
			if i == 1 {
				k = ""
			}
			res[i] = f.confirm(c, k, s.Num("bill_version"), s.Num("total_satang"))
		}()
	}
	wg.Wait()
	won := 0
	for _, r := range res {
		switch {
		case r.Status == 201:
			won++
		case r.Code() == "ALREADY_PAID" && r.Str("error", "fields", "receipt_reference") != "":
		default:
			t.Fatalf("confirm outcome: %d %s", r.Status, r.Raw)
		}
	}
	if won != 1 || f.e.Count("SELECT count(*) FROM settlements WHERE visit_id = $1", f.visit) != 1 {
		t.Fatalf("winners = %d", won)
	}
	// The first cashier lost the response and retries with the same key.
	retry := f.confirm(f.e.Cashier, key, s.Num("bill_version"), s.Num("total_satang"))
	if res[0].Status == 201 && (retry.Status != 201 || retry.Header.Get("Idempotency-Replayed") != "true" ||
		retry.Str("receipt_reference") != res[0].Str("receipt_reference")) {
		t.Fatalf("replay: %d %s", retry.Status, retry.Raw)
	}
	if n := f.e.Count("SELECT count(*) FROM audit_events WHERE action = 'settlement.confirmed'"); n != 1 {
		t.Fatalf("confirm audit rows = %d", n)
	}
}

// TestGuestCannotSettle (BIL-A4): guest credentials and uploads never settle.
func TestGuestCannotSettle(t *testing.T) {
	f := setup(t)
	id, v := f.order(f.tea, 1)
	f.serve(id, v)
	g := f.e.Diner(f.token)
	bv := f.bill(f.e.Cashier).Num("bill_version")
	// Guest sessions live on the PWA origin: the staff origin guard or the
	// missing staff session refuses them (403/401), never a settlement.
	if r := g.Send(f.path("/settlement/begin"), "", map[string]any{"expected_version": bv}); r.Status != 401 && r.Status != 403 {
		t.Fatalf("guest begin: %d", r.Status)
	}
	s := f.begin(f.e.Cashier, bv)
	r := g.Send(f.path("/settlement/confirm"), "", map[string]any{"expected_version": s.Num("bill_version"),
		"amount_satang": s.Num("total_satang"), "method": "bank_transfer", "verification_note": "slip attached"})
	if r.Status != 401 && r.Status != 403 {
		t.Fatalf("guest confirm: %d %s", r.Status, r.Raw)
	}
	if r := f.confirm(f.e.Host, "", s.Num("bill_version"), s.Num("total_satang")); r.Status != 403 {
		t.Fatalf("host confirm: %d", r.Status)
	}
	if f.e.Count("SELECT count(*) FROM settlements") != 0 {
		t.Fatal("a settlement exists")
	}
}

// TestConfirmEffects: exact amount, validation, access revoked, table kept
// until departure, then paid → depart → cleaning → ready (SEA-004).
func TestConfirmEffects(t *testing.T) {
	f := setup(t)
	id, v := f.order(f.tea, 1)
	f.serve(id, v)
	phone := f.e.Diner(f.token) // a diner session from before payment
	s := f.begin(f.e.Cashier, f.bill(f.e.Cashier).Num("bill_version"))
	for _, bad := range []map[string]any{
		{"expected_version": s.Num("bill_version"), "amount_satang": 6000, "method": "crypto", "verification_note": "x"},
		{"expected_version": s.Num("bill_version"), "amount_satang": 6000, "method": "cash", "verification_note": "  "},
	} {
		if r := f.e.StaffSend("POST", f.e.Cashier, f.path("/settlement/confirm"), bad); r.Status != 422 {
			t.Fatalf("invalid confirm: %d %s", r.Status, r.Raw)
		}
	}
	c := f.confirm(f.e.Cashier, "", s.Num("bill_version"), 6000)
	if c.Status != 201 || c.Num("amount_satang") != 6000 || c.Str("bill", "visit_state") != "paid" {
		t.Fatalf("confirm: %d %s", c.Status, c.Raw)
	}
	ref := c.Str("receipt_reference")
	if len(ref) != 12 || ref[:2] != "R-" {
		t.Fatalf("receipt reference %q", ref)
	}
	if r := phone.Get(f.path("/orders")); r.Status != 401 {
		t.Fatalf("dining session survived payment: %d", r.Status)
	}
	if r := f.e.Do("POST", "/api/v1/sessions/capability", "", testenv.PWAOrigin, "", map[string]any{"token": f.token, "kind": "visit"}); r.Code() != "TOKEN_INVALID" {
		t.Fatalf("dining QR still exchanges after payment: %d %s", r.Status, r.Raw)
	}
	if r := f.e.StaffSend("POST", f.e.Cashier, "/api/v1/bills/resolve", map[string]any{"dining_token": f.token}); r.Status != 404 {
		t.Fatalf("revoked token resolved: %d", r.Status)
	}
	if n := f.e.Count("SELECT count(*) FROM table_claims WHERE table_id = $1", f.table); n != 1 {
		t.Fatalf("paid table released: claims = %d", n)
	}
	if b := f.bill(f.e.Cashier); b.Str("settlement", "receipt_reference") != ref || b.Get("frozen") != true {
		t.Fatalf("paid bill: %s", b.Raw)
	}
	visitVersion := testenv.Scalar[int](f.e, "SELECT version FROM visits WHERE id = $1", f.visit)
	if r := f.e.StaffSend("POST", f.e.Host, f.path("/depart"), map[string]any{"expected_version": visitVersion}); r.Status != 200 {
		t.Fatalf("depart: %d %s", r.Status, r.Raw)
	}
	tv := testenv.Scalar[int](f.e, "SELECT version FROM dining_tables WHERE id = $1", f.table)
	if r := f.e.StaffSend("POST", f.e.Host, "/api/v1/tables/"+f.table+"/ready", map[string]any{"expected_version": tv}); r.Status != 200 {
		t.Fatalf("ready: %d %s", r.Status, r.Raw)
	}
	if b := f.bill(f.e.Cashier); b.Str("visit_state") != "departed" || b.Num("total_satang") != 6000 {
		t.Fatalf("departed bill: %s", b.Raw)
	}
}

// TestBillStatementsConstant: a bill read costs the same statements for 1 or
// 30 lines (no per-line queries).
func TestBillStatementsConstant(t *testing.T) {
	f := setup(t)
	f.order(f.tea, 1)
	one := f.e.Statements(func() { f.bill(f.e.Cashier) })
	for range 29 {
		f.order(f.rice, 1)
	}
	many := f.e.Statements(func() { f.bill(f.e.Cashier) })
	if one != many || many > 6 {
		t.Fatalf("bill statements: %d for 1 line, %d for 30", one, many)
	}
}
