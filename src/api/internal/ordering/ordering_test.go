package ordering_test

import (
	"strings"
	"sync"
	"testing"

	"github.com/epic-rinn/tableflow/src/api/internal/testenv"
)

// fixture is a seated visit with a small menu.
type fixture struct {
	e        *testenv.Env
	visit    string
	token    string
	revision int
	pad      string // item with required protein group
	chicken  string
	shrimp   string
	egg      string
	tea      string // item without options
}

func setup(t *testing.T) *fixture {
	e := testenv.New(t)
	opt := func(th, en string, d int) map[string]any {
		return map[string]any{"name_th": th, "name_en": en, "price_delta_satang": d}
	}
	r := e.Do("PUT", "/api/v1/branches/"+e.Branch+"/menu", testenv.StaffCookie(e.Manager), testenv.AdminOrigin, "", map[string]any{
		"expected_revision": 1,
		"categories": []map[string]any{{"name_th": "อาหาร", "name_en": "Food", "items": []map[string]any{
			{"name_th": "ผัดไทย", "name_en": "Pad Thai", "price_satang": 12000, "option_groups": []map[string]any{
				{"name_th": "เนื้อ", "name_en": "Protein", "min_choices": 1, "max_choices": 1, "options": []map[string]any{opt("ไก่", "Chicken", 0), opt("กุ้ง", "Shrimp", 3000)}},
				{"name_th": "เพิ่ม", "name_en": "Extras", "min_choices": 0, "max_choices": 2, "options": []map[string]any{opt("ไข่", "Egg", 1000), opt("ถั่ว", "Peanuts", 500)}},
			}},
			{"name_th": "ชาไทย", "name_en": "Thai Tea", "price_satang": 6000, "option_groups": []map[string]any{}},
		}}},
	})
	if r.Status != 200 {
		t.Fatalf("menu: %d %s", r.Status, r.Raw)
	}
	f := &fixture{e: e, revision: int(r.Num("revision"))}
	f.pad = r.Str("categories", 0, "items", 0, "id")
	f.chicken = r.Str("categories", 0, "items", 0, "option_groups", 0, "options", 0, "id")
	f.shrimp = r.Str("categories", 0, "items", 0, "option_groups", 0, "options", 1, "id")
	f.egg = r.Str("categories", 0, "items", 0, "option_groups", 1, "options", 0, "id")
	f.tea = r.Str("categories", 0, "items", 1, "id")
	f.visit, f.token = e.SeatWalkIn(e.Table("T1", 4), 3)
	return f
}

func (f *fixture) order(lines ...map[string]any) map[string]any {
	return map[string]any{"menu_revision": f.revision, "lines": lines}
}

func line(item string, qty int, opts ...string) map[string]any {
	if opts == nil {
		opts = []string{}
	}
	return map[string]any{"item_id": item, "quantity": qty, "option_ids": opts}
}

func (f *fixture) ordersPath() string { return "/api/v1/visits/" + f.visit + "/orders" }

// TestTwoPhonesOrderOnce (ORD-A1, ORD-005): two phones' orders appear once
// each on the shared visit; drafts are never shared.
func TestTwoPhonesOrderOnce(t *testing.T) {
	f := setup(t)
	a, b := f.e.Diner(f.token), f.e.Diner(f.token)
	ra := a.Send(f.ordersPath(), "", f.order(line(f.pad, 2, f.shrimp, f.egg)))
	rb := b.Send(f.ordersPath(), "", f.order(line(f.tea, 1)))
	if ra.Status != 201 || rb.Status != 201 {
		t.Fatalf("orders: %d %s / %d %s", ra.Status, ra.Raw, rb.Status, rb.Raw)
	}
	if ra.Num("lines", 0, "unit_price_satang") != 16000 || ra.Num("lines", 0, "line_total_satang") != 32000 || ra.Str("actor") != "guest" {
		t.Fatalf("snapshot: %s", ra.Raw)
	}
	for _, g := range []*testenv.Guest{a, b} {
		r := g.Get(f.ordersPath())
		if r.Status != 200 || r.Len("items") != 2 || r.Num("chargeable_total_satang") != 38000 || r.Header.Get("Cache-Control") != "private, no-store" {
			t.Fatalf("shared view: %d %s", r.Status, r.Raw)
		}
	}
	if n := f.e.Count("SELECT count(*) FROM order_lines WHERE visit_id = $1", f.visit); n != 2 {
		t.Fatalf("lines = %d", n)
	}
}

// TestOrderRetryAndConflict (ORD-A2, ORD-003).
func TestOrderRetryAndConflict(t *testing.T) {
	f := setup(t)
	g := f.e.Diner(f.token)
	key := testenv.NewKey()
	body := f.order(line(f.tea, 2))
	const n = 5
	res := make([]testenv.Resp, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range n {
		wg.Go(func() { <-start; res[i] = g.Send(f.ordersPath(), key, body) })
	}
	close(start)
	wg.Wait()
	for i, r := range res {
		if r.Status != 201 || r.Str("id") != res[0].Str("id") {
			t.Fatalf("retry %d: %d %s", i, r.Status, r.Raw)
		}
	}
	if n := f.e.Count("SELECT count(*) FROM order_lines WHERE visit_id = $1", f.visit); n != 1 {
		t.Fatalf("kitchen lines after retries = %d", n)
	}
	if r := g.Send(f.ordersPath(), key, f.order(line(f.tea, 3))); r.Code() != "IDEMPOTENCY_CONFLICT" {
		t.Fatalf("same key, different body: %d %s", r.Status, r.Raw)
	}
}

// TestStaleMenuRejectsWholeOrder (ORD-A3, ORD-002).
func TestStaleMenuRejectsWholeOrder(t *testing.T) {
	f := setup(t)
	g := f.e.Diner(f.token)
	v := testenv.Scalar[int](f.e, "SELECT version FROM menu_items WHERE id = $1", f.tea)
	if r := f.e.Do("PATCH", "/api/v1/menu-items/"+f.tea+"/availability", testenv.StaffCookie(f.e.Kitchen), testenv.AdminOrigin, "",
		map[string]any{"expected_version": v, "sold_out": true}); r.Status != 200 {
		t.Fatalf("sold out: %d", r.Status)
	}
	r := g.Send(f.ordersPath(), "", f.order(line(f.pad, 1, f.chicken), line(f.tea, 1)))
	if r.Status != 409 || r.Code() != "MENU_CHANGED" || r.Get("error", "fields", f.tea) == nil || r.Get("error", "fields", f.pad) != nil {
		t.Fatalf("sold-out item: %d %s", r.Status, r.Raw)
	}
	if n := f.e.Count("SELECT count(*) FROM orders"); n != 0 {
		t.Fatal("partial order created")
	}
	// A reprice after browsing also fails; an unrelated change does not.
	f.e.Exec("UPDATE menu_items SET price_satang = 13000, changed_revision = changed_revision + 10 WHERE id = $1", f.pad)
	if r := g.Send(f.ordersPath(), "", f.order(line(f.pad, 1, f.chicken))); r.Code() != "MENU_CHANGED" {
		t.Fatalf("repriced item: %d %s", r.Status, r.Raw)
	}
	cur := testenv.Scalar[int](f.e, "SELECT revision FROM menus WHERE branch_id = $1", f.e.Branch)
	fresh := map[string]any{"menu_revision": cur, "lines": []map[string]any{line(f.tea, 1)}}
	if r := g.Send(f.ordersPath(), "", fresh); r.Code() != "MENU_CHANGED" {
		t.Fatalf("sold-out item still orderable: %d", r.Status)
	}
}

// TestMenuChangeVersusSubmission: a sold-out toggle racing an order either
// commits after the order (order stands) or before (order rejected); never a
// half-applied state.
func TestMenuChangeVersusSubmission(t *testing.T) {
	for round := range 5 {
		f := setup(t)
		g := f.e.Diner(f.token)
		v := testenv.Scalar[int](f.e, "SELECT version FROM menu_items WHERE id = $1", f.tea)
		var order, toggle testenv.Resp
		var wg sync.WaitGroup
		start := make(chan struct{})
		wg.Go(func() { <-start; order = g.Send(f.ordersPath(), "", f.order(line(f.tea, 1))) })
		wg.Go(func() {
			<-start
			toggle = f.e.Do("PATCH", "/api/v1/menu-items/"+f.tea+"/availability", testenv.StaffCookie(f.e.Kitchen), testenv.AdminOrigin, "",
				map[string]any{"expected_version": v, "sold_out": true})
		})
		close(start)
		wg.Wait()
		if toggle.Status != 200 {
			t.Fatalf("round %d toggle: %d", round, toggle.Status)
		}
		lines := f.e.Count("SELECT count(*) FROM order_lines")
		switch order.Status {
		case 201:
			if lines != 1 {
				t.Fatalf("round %d: order ok but %d lines", round, lines)
			}
		case 409:
			if lines != 0 || order.Code() != "MENU_CHANGED" {
				t.Fatalf("round %d: rejected with %s, %d lines", round, order.Code(), lines)
			}
		default:
			t.Fatalf("round %d: %d %s", round, order.Status, order.Raw)
		}
	}
}

// TestOrderValidation (ORD-001 bounds, option rules).
func TestOrderValidation(t *testing.T) {
	f := setup(t)
	g := f.e.Diner(f.token)
	cases := map[string]map[string]any{
		"quantity 0":        f.order(line(f.tea, 0)),
		"quantity 21":       f.order(line(f.tea, 21)),
		"required missing":  f.order(line(f.pad, 1)),
		"too many in group": f.order(line(f.pad, 1, f.chicken, f.shrimp)),
		"foreign option":    f.order(line(f.tea, 1, f.egg)),
		"duplicate option":  f.order(line(f.pad, 1, f.chicken, f.chicken)),
		"no lines":          f.order(),
		"future revision":   {"menu_revision": f.revision + 5, "lines": []map[string]any{line(f.tea, 1)}},
		"long note":         f.order(map[string]any{"item_id": f.tea, "quantity": 1, "option_ids": []string{}, "note": strings.Repeat("x", 501)}),
		"unknown item":      f.order(line("0198f0c0-0000-7000-8000-00000000beef", 1)),
	}
	for name, body := range cases {
		if r := g.Send(f.ordersPath(), "", body); r.Status != 422 {
			t.Errorf("%s: %d %s", name, r.Status, r.Raw)
		}
	}
	many := []map[string]any{}
	for range 51 {
		many = append(many, line(f.tea, 1))
	}
	if r := g.Send(f.ordersPath(), "", f.order(many...)); r.Status != 422 {
		t.Errorf("51 lines: %d", r.Status)
	}
	if n := f.e.Count("SELECT count(*) FROM orders"); n != 0 {
		t.Fatalf("invalid orders created %d rows", n)
	}
}

// TestOrderAccessControl: other visits, the wrong origin and anonymous
// callers are denied.
func TestOrderAccessControl(t *testing.T) {
	f := setup(t)
	otherVisit, otherToken := f.e.SeatWalkIn(f.e.Table("T2", 4), 2)
	mine := f.e.Diner(f.token)
	theirs := f.e.Diner(otherToken)
	if r := theirs.Send(f.ordersPath(), "", f.order(line(f.tea, 1))); r.Status != 404 {
		t.Fatalf("cross-visit order: %d", r.Status)
	}
	if r := theirs.Get(f.ordersPath()); r.Status != 404 {
		t.Fatalf("cross-visit read: %d", r.Status)
	}
	if r := f.e.Do("POST", f.ordersPath(), "__Host-tf_guest="+mine.Sess, testenv.AdminOrigin, testenv.NewKey(), f.order(line(f.tea, 1))); r.Status != 403 {
		t.Fatalf("guest via admin origin: %d", r.Status)
	}
	if r := f.e.Do("GET", f.ordersPath(), "", "", "", nil); r.Status != 401 {
		t.Fatalf("anonymous read: %d", r.Status)
	}
	if r := f.e.StaffGet(f.e.Kitchen, "/api/v1/visits/"+otherVisit+"/orders"); r.Status != 200 {
		t.Fatalf("branch staff read: %d", r.Status)
	}
	if r := f.e.StaffSend("POST", f.e.Kitchen, f.ordersPath(), f.order(line(f.tea, 1))); r.Status != 403 {
		t.Fatalf("kitchen placed an order: %d", r.Status)
	}
}

// TestAssistedOrderRecordsStaff (ORD-005).
func TestAssistedOrderRecordsStaff(t *testing.T) {
	f := setup(t)
	r := f.e.StaffSend("POST", f.e.Host, f.ordersPath(), f.order(line(f.pad, 1, f.chicken)))
	if r.Status != 201 || r.Str("actor") != "staff" {
		t.Fatalf("assisted order: %d %s", r.Status, r.Raw)
	}
	if n := f.e.Count("SELECT count(*) FROM orders WHERE actor_kind = 'staff' AND actor_staff_id IS NOT NULL"); n != 1 {
		t.Fatal("staff actor not recorded")
	}
}

// TestOrdersOnlyOnOpenVisits (ORD-007 gating) and the close-empty guard.
func TestOrdersOnlyOnOpenVisits(t *testing.T) {
	f := setup(t)
	g := f.e.Diner(f.token)
	g.Send(f.ordersPath(), "", f.order(line(f.tea, 1)))
	v := testenv.Scalar[int](f.e, "SELECT version FROM visits WHERE id = $1", f.visit)
	if r := f.e.StaffSend("POST", f.e.Host, "/api/v1/visits/"+f.visit+"/close-empty", map[string]any{"expected_version": v, "reason": "left"}); r.Code() != "VISIT_HAS_ORDERS" {
		t.Fatalf("close-empty with orders: %d %s", r.Status, r.Raw)
	}
	f.e.Exec("UPDATE visits SET state = 'paid', paid_at = now() WHERE id = $1", f.visit)
	if r := g.Send(f.ordersPath(), "", f.order(line(f.tea, 1))); r.Code() != "VISIT_STATE_CONFLICT" {
		t.Fatalf("order on paid visit: %d %s", r.Status, r.Raw)
	}
}

// TestOrderReadStatementCount: the order view uses three statements
// (authorisation, page, lines) plus totals regardless of order count.
func TestOrderReadStatementCount(t *testing.T) {
	f := setup(t)
	g := f.e.Diner(f.token)
	for range 10 {
		g.Send(f.ordersPath(), "", f.order(line(f.tea, 1), line(f.pad, 1, f.chicken)))
	}
	var r testenv.Resp
	n := f.e.Statements(func() { r = g.Get(f.ordersPath()) })
	if r.Len("items") != 10 || r.Num("chargeable_lines") != 20 {
		t.Fatalf("read: %s", r.Raw)
	}
	// 1 guest authentication + visit check + page + lines + totals.
	if n != 5 {
		t.Fatalf("order read used %d statements, want 5", n)
	}
}
