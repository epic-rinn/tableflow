package reporting_test

import (
	"strings"
	"testing"
	"time"

	"github.com/epic-rinn/tableflow/src/api/internal/testenv"
)

type fixture struct {
	e        *testenv.Env
	revision int
	tea      string
	today    string
}

func setup(t *testing.T) *fixture {
	e := testenv.New(t)
	r := e.Do("PUT", "/api/v1/branches/"+e.Branch+"/menu", testenv.StaffCookie(e.Manager), testenv.AdminOrigin, "", map[string]any{
		"expected_revision": 1,
		"categories": []map[string]any{{"name_th": "เครื่องดื่ม", "name_en": "Drinks", "items": []map[string]any{
			{"name_th": "ชาไทย", "name_en": "Thai Tea", "price_satang": 6000, "option_groups": []map[string]any{}},
		}}},
	})
	if r.Status != 200 {
		t.Fatalf("menu: %d %s", r.Status, r.Raw)
	}
	loc, _ := time.LoadLocation("Asia/Bangkok")
	return &fixture{e: e, revision: int(r.Num("revision")), tea: r.Str("categories", 0, "items", 0, "id"), today: time.Now().In(loc).Format(time.DateOnly)}
}

// paid seats a party, orders qty teas, serves them and settles with method;
// an optional member cookie claims the visit first. Returns the settlement ID.
func (f *fixture) paid(label string, qty int, method, memberCookie string) string {
	f.e.T.Helper()
	visit, token := f.e.SeatWalkIn(f.e.Table(label, 4), 2)
	g := f.e.Diner(token)
	r := g.Send("/api/v1/visits/"+visit+"/orders", "", map[string]any{"menu_revision": f.revision,
		"lines": []map[string]any{{"item_id": f.tea, "quantity": qty, "option_ids": []string{}}}})
	if r.Status != 201 {
		f.e.T.Fatalf("order: %s", r.Raw)
	}
	f.e.Exec("UPDATE order_lines SET state = 'served', version = version + 1 WHERE visit_id = $1", visit)
	bill := func() testenv.Resp { return f.e.StaffGet(f.e.Cashier, "/api/v1/visits/"+visit+"/bill") }
	if memberCookie != "" {
		if c := g.WithMember(memberCookie).Send("/api/v1/visits/"+visit+"/member-claim", "", map[string]any{"expected_version": bill().Num("bill_version")}); c.Status != 200 {
			f.e.T.Fatalf("claim: %s", c.Raw)
		}
	}
	s := f.e.StaffSend("POST", f.e.Cashier, "/api/v1/visits/"+visit+"/settlement/begin", map[string]any{"expected_version": bill().Num("bill_version")})
	c := f.e.StaffSend("POST", f.e.Cashier, "/api/v1/visits/"+visit+"/settlement/confirm", map[string]any{
		"expected_version": s.Num("bill_version"), "amount_satang": s.Num("total_satang"), "method": method, "verification_note": "test"})
	if c.Status != 201 {
		f.e.T.Fatalf("confirm: %d %s", c.Status, c.Raw)
	}
	return c.Str("id")
}

func (f *fixture) report(cookie, from, to string) testenv.Resp {
	return f.e.StaffGet(cookie, "/api/v1/branches/"+f.e.Branch+"/reports/daily?from="+from+"&to="+to)
}

// TestDailyReportReconciles (OPS-002): totals equal settlements, refunds and
// the ledger; methods, tickets, visits and points are counted.
func TestDailyReportReconciles(t *testing.T) {
	f := setup(t)
	_, member := f.e.Member("ann@example.com")
	f.paid("A", 2, "cash", "")         // 120.00
	card := f.paid("B", 1, "card", "") // 60.00, refunded below
	f.paid("C", 3, "cash", member)     // 180.00 → 1 point
	if r := f.e.StaffSend("POST", f.e.Manager, "/api/v1/settlements/"+card+"/refund", map[string]any{"reason": "cold", "external_reference": "REF-1"}); r.Status != 201 {
		t.Fatalf("refund: %s", r.Raw)
	}
	for range 2 {
		if r := f.e.StaffSend("POST", f.e.Host, "/api/v1/branches/"+f.e.Branch+"/queue-tickets", map[string]any{"party_size": 2, "needs": []string{}}); r.Status != 201 {
			t.Fatalf("join: %s", r.Raw)
		}
	}
	ticket := testenv.Scalar[string](f.e, "SELECT id::text FROM queue_tickets ORDER BY join_order LIMIT 1")
	if r := f.e.StaffSend("POST", f.e.Host, "/api/v1/queue-tickets/"+ticket+"/cancel", map[string]any{"expected_version": 1}); r.Status != 200 {
		t.Fatalf("cancel: %s", r.Raw)
	}

	r := f.report(f.e.Manager, f.today, f.today)
	if r.Status != 200 || r.Len("days") != 1 || r.Str("timezone") != "Asia/Bangkok" || r.Header.Get("Cache-Control") != "private, no-store" {
		t.Fatalf("report: %d %s", r.Status, r.Raw)
	}
	tot := func(path ...any) int64 { return r.Num(append([]any{"totals"}, path...)...) }
	checks := map[string][2]int64{
		"sales count":        {tot("sales", "count"), 3},
		"sales":              {tot("sales", "satang"), 36000},
		"cash":               {tot("sales_by_method", "cash", "satang"), 30000},
		"card":               {tot("sales_by_method", "card", "satang"), 6000},
		"refunds":            {tot("refunds", "satang"), 6000},
		"refunds card":       {tot("refunds_by_method", "card", "count"), 1},
		"net":                {tot("net_satang"), 30000},
		"visits":             {tot("visits_opened"), 3},
		"tickets":            {tot("tickets_joined"), 2},
		"cancelled":          {tot("tickets_cancelled"), 1},
		"points":             {tot("points_earned"), 1},
		"member settlements": {tot("member_settlements"), 1},
		"day sales":          {r.Num("days", 0, "sales", "satang"), 36000},
	}
	for name, c := range checks {
		if c[0] != c[1] {
			t.Errorf("%s = %d, want %d", name, c[0], c[1])
		}
	}
	// Reconciles against the source tables directly.
	if db := testenv.Scalar[int64](f.e, "SELECT sum(amount_satang)::bigint FROM settlements"); db != tot("sales", "satang") {
		t.Fatalf("sales %d vs settlements %d", tot("sales", "satang"), db)
	}
	if db := testenv.Scalar[int64](f.e, "SELECT sum(points_delta)::bigint FROM loyalty_ledger WHERE kind = 'earn'"); db != tot("points_earned") {
		t.Fatalf("points %d vs ledger %d", tot("points_earned"), db)
	}
	// Report drill-down: receipts of the same business date.
	list := f.e.StaffGet(f.e.Manager, "/api/v1/branches/"+f.e.Branch+"/settlements?from="+f.today+"&to="+f.today)
	if list.Len("items") != 3 {
		t.Fatalf("drill-down: %s", list.Raw)
	}
	yesterday := time.Now().Add(-48 * time.Hour).Format(time.DateOnly)
	if l := f.e.StaffGet(f.e.Manager, "/api/v1/branches/"+f.e.Branch+"/settlements?from="+yesterday+"&to="+yesterday); l.Len("items") != 0 {
		t.Fatalf("drill-down outside range: %s", l.Raw)
	}
}

// TestReportBusinessDate: days follow Asia/Bangkok midnight, not UTC.
func TestReportBusinessDate(t *testing.T) {
	f := setup(t)
	a := f.paid("A", 1, "cash", "")
	b := f.paid("B", 1, "cash", "")
	// 2026-09-10 16:59:59Z is 23:59:59 on 09-10 in Bangkok; 17:00:01Z is 09-11.
	f.e.Exec("UPDATE settlements SET paid_at = '2026-09-10T16:59:59Z' WHERE id = $1", a)
	f.e.Exec("UPDATE settlements SET paid_at = '2026-09-10T17:00:01Z' WHERE id = $1", b)
	r := f.report(f.e.Manager, "2026-09-10", "2026-09-11")
	if r.Len("days") != 2 || r.Str("days", 0, "date") != "2026-09-10" || r.Num("days", 0, "sales", "count") != 1 || r.Num("days", 1, "sales", "count") != 1 {
		t.Fatalf("business dates: %s", r.Raw)
	}
}

// TestReportBoundsAndRoles: 31-day cap, validation, manager only.
func TestReportBoundsAndRoles(t *testing.T) {
	f := setup(t)
	if r := f.report(f.e.Cashier, f.today, f.today); r.Status != 403 {
		t.Fatalf("cashier report: %d", r.Status)
	}
	if r := f.e.StaffGet(f.e.Manager, "/api/v1/branches/00000000-0000-4000-8000-000000000000/reports/daily?from="+f.today+"&to="+f.today); r.Status != 404 {
		t.Fatalf("other branch: %d", r.Status)
	}
	for _, q := range [][2]string{{"2026-09-01", "2026-10-02"}, {"2026-09-02", "2026-09-01"}, {"yesterday", "today"}, {"", ""}} {
		if r := f.report(f.e.Manager, q[0], q[1]); r.Status != 422 {
			t.Fatalf("range %v: %d", q, r.Status)
		}
	}
	if r := f.report(f.e.Manager, "2026-09-01", "2026-10-01"); r.Status != 200 || r.Len("days") != 31 {
		t.Fatalf("31 days: %d", r.Status)
	}
	if r := f.e.StaffGet(f.e.Manager, "/api/v1/branches/"+f.e.Branch+"/settlements?from=2026-09-01&to=2026-10-02"); r.Status != 422 {
		t.Fatalf("drill-down cap: %d", r.Status)
	}
}

// TestAuditEventsViewer (OPS-001): manager-only, filtered, paginated and
// redacted to known detail keys.
func TestAuditEventsViewer(t *testing.T) {
	f := setup(t)
	sid := f.paid("A", 1, "cash", "")
	if r := f.e.StaffSend("POST", f.e.Manager, "/api/v1/settlements/"+sid+"/refund", map[string]any{"reason": "cold", "external_reference": "REF-1"}); r.Status != 201 {
		t.Fatalf("refund: %s", r.Raw)
	}
	// A hypothetical future mistake: a secret in details must not surface.
	f.e.Exec(`INSERT INTO audit_events (branch_id, action, resource_type, request_id, details)
		VALUES ($1, 'test.leak', 'test', 'req', '{"token": "raw-secret-token", "password": "hunter2", "amount_satang": 5}')`, f.e.Branch)
	path := "/api/v1/branches/" + f.e.Branch + "/audit-events?from=" + f.today + "&to=" + f.today
	if r := f.e.StaffGet(f.e.Cashier, path); r.Status != 403 {
		t.Fatalf("cashier audit: %d", r.Status)
	}
	all := f.e.StaffGet(f.e.Manager, path+"&limit=100")
	if all.Status != 200 || all.Len("items") < 3 {
		t.Fatalf("audit: %d %s", all.Status, all.Raw)
	}
	if strings.Contains(all.Raw, "raw-secret-token") || strings.Contains(all.Raw, "hunter2") || strings.Contains(all.Raw, "token_hash") {
		t.Fatal("audit viewer exposed secret material")
	}
	leak := all.Get("items", 0) // newest first
	if m, _ := leak.(map[string]any); m["action"] != "test.leak" || m["details"].(map[string]any)["amount_satang"] != float64(5) || len(m["details"].(map[string]any)) != 1 {
		t.Fatalf("redaction: %v", leak)
	}
	settle := f.e.StaffGet(f.e.Manager, path+"&action=settlement")
	for i := range settle.Len("items") {
		if a := settle.Str("items", i, "action"); !strings.HasPrefix(a, "settlement.") {
			t.Fatalf("filter returned %s", a)
		}
	}
	if settle.Len("items") != 3 || settle.Str("items", 0, "action") != "settlement.refunded" || settle.Str("items", 0, "reason") != "cold" ||
		settle.Str("items", 0, "actor") != "Manager" {
		t.Fatalf("settlement events: %s", settle.Raw)
	}
	seen := map[string]bool{}
	cursor := ""
	for range 20 {
		p := path + "&limit=2"
		if cursor != "" {
			p += "&cursor=" + cursor
		}
		r := f.e.StaffGet(f.e.Manager, p)
		for i := range r.Len("items") {
			seen[r.Str("items", i, "id")] = true
		}
		if cursor = r.Str("next_cursor"); cursor == "" {
			break
		}
	}
	if len(seen) != all.Len("items") {
		t.Fatalf("paged %d of %d", len(seen), all.Len("items"))
	}
	if r := f.e.StaffGet(f.e.Manager, path+"&action=DROP%20TABLE"); r.Status != 422 {
		t.Fatalf("bad action filter: %d", r.Status)
	}
}

// TestReportingContractConformance: report and audit responses match OpenAPI.
func TestReportingContractConformance(t *testing.T) {
	f := setup(t)
	f.paid("A", 1, "cash", "")
	testenv.CheckContract(t, "GET", "/api/v1/branches/{branch_id}/reports/daily", f.report(f.e.Manager, f.today, f.today))
	testenv.CheckContract(t, "GET", "/api/v1/branches/{branch_id}/audit-events",
		f.e.StaffGet(f.e.Manager, "/api/v1/branches/"+f.e.Branch+"/audit-events?from="+f.today+"&to="+f.today))
	testenv.CheckContract(t, "GET", "/api/v1/branches/{branch_id}/reports/daily", f.report(f.e.Manager, "x", "y"))
}
