package ordering_test

import (
	"fmt"
	"testing"

	"github.com/epic-rinn/tableflow/src/api/internal/testenv"
)

func (f *fixture) placeLine() (string, int) {
	f.e.T.Helper()
	r := f.e.Diner(f.token).Send(f.ordersPath(), "", f.order(line(f.tea, 1)))
	if r.Status != 201 {
		f.e.T.Fatalf("order: %d %s", r.Status, r.Raw)
	}
	return r.Str("lines", 0, "id"), int(r.Num("lines", 0, "version"))
}

func (f *fixture) move(cookie, lineID string, version int, to string, reason ...string) testenv.Resp {
	body := map[string]any{"expected_version": version, "to_state": to}
	if len(reason) > 0 {
		body["reason"] = reason[0]
	}
	return f.e.StaffSend("POST", cookie, "/api/v1/order-lines/"+lineID+"/transition", body)
}

// TestLineTransitionMatrix (ORD-004): roles and allowed steps.
func TestLineTransitionMatrix(t *testing.T) {
	f := setup(t)
	id, v := f.placeLine()
	steps := []struct {
		cookie, to string
		want       int
	}{
		{f.e.Host, "accepted", 403},
		{f.e.Kitchen, "preparing", 409},
		{f.e.Kitchen, "accepted", 200},
		{f.e.Kitchen, "preparing", 200},
		{f.e.Kitchen, "ready", 200},
		{f.e.Host, "served", 200},
		{f.e.Kitchen, "accepted", 409},
	}
	for i, s := range steps {
		r := f.move(s.cookie, id, v, s.to)
		if r.Status != s.want {
			t.Fatalf("step %d → %s: %d %s", i, s.to, r.Status, r.Raw)
		}
		if r.Status == 200 {
			v = int(r.Num("version"))
		}
	}
	board := f.e.StaffGet(f.e.Kitchen, "/api/v1/branches/"+f.e.Branch+"/kitchen-lines")
	if board.Status != 200 || board.Len("items") != 0 {
		t.Fatalf("served line still on board: %s", board.Raw)
	}
	if r := f.e.StaffGet(f.e.Host, "/api/v1/branches/"+f.e.Branch+"/kitchen-lines"); r.Status != 200 {
		t.Fatalf("host board read: %d", r.Status)
	}
}

// TestRejectedLineExcludedFromTotal (ORD-A5).
func TestRejectedLineExcludedFromTotal(t *testing.T) {
	f := setup(t)
	id, v := f.placeLine()
	f.placeLine()
	if r := f.move(f.e.Kitchen, id, v, "rejected"); r.Status != 422 {
		t.Fatalf("reject without reason: %d", r.Status)
	}
	bill := testenv.Scalar[int](f.e, "SELECT bill_version FROM visits WHERE id = $1", f.visit)
	r := f.move(f.e.Kitchen, id, v, "rejected", "out of tea leaves")
	if r.Status != 200 || r.Get("chargeable") != false || r.Str("reason") != "out of tea leaves" {
		t.Fatalf("reject: %d %s", r.Status, r.Raw)
	}
	orders := f.e.Diner(f.token).Get(f.ordersPath())
	if orders.Num("chargeable_total_satang") != 6000 || orders.Num("chargeable_lines") != 1 {
		t.Fatalf("totals after rejection: %s", orders.Raw)
	}
	if testenv.Scalar[int](f.e, "SELECT bill_version FROM visits WHERE id = $1", f.visit) != bill+1 {
		t.Fatal("rejection did not bump the bill version")
	}
}

// TestLateCancellationNeedsManager (ORD-004): hosts cancel unprepared lines;
// preparing and later needs a manager and is audited.
func TestLateCancellationNeedsManager(t *testing.T) {
	f := setup(t)
	early, ev := f.placeLine()
	if r := f.move(f.e.Host, early, ev, "cancelled", "guest changed mind"); r.Status != 200 {
		t.Fatalf("host cancels submitted: %d %s", r.Status, r.Raw)
	}
	late, lv := f.placeLine()
	for _, to := range []string{"accepted", "preparing"} {
		r := f.move(f.e.Kitchen, late, lv, to)
		lv = int(r.Num("version"))
	}
	if r := f.move(f.e.Host, late, lv, "cancelled", "wrong dish"); r.Status != 403 {
		t.Fatalf("host cancelled a preparing line: %d", r.Status)
	}
	if r := f.move(f.e.Kitchen, late, lv, "cancelled", "wrong dish"); r.Status != 403 {
		t.Fatalf("kitchen cancelled: %d", r.Status)
	}
	if r := f.move(f.e.Manager, late, lv, "cancelled", "wrong dish"); r.Status != 200 {
		t.Fatalf("manager cancel: %d %s", r.Status, r.Raw)
	}
	if n := f.e.Count("SELECT count(*) FROM audit_events WHERE action = 'order_line.cancelled_late' AND reason = 'wrong dish'"); n != 1 {
		t.Fatalf("late cancellation audit rows = %d", n)
	}
}

// TestPaidVisitRejectsFinancialChanges (ORD-007).
func TestPaidVisitRejectsFinancialChanges(t *testing.T) {
	f := setup(t)
	id, v := f.placeLine()
	f.e.Exec("UPDATE visits SET state = 'paid', paid_at = now() WHERE id = $1", f.visit)
	if r := f.move(f.e.Manager, id, v, "cancelled", "late"); r.Code() != "VISIT_STATE_CONFLICT" {
		t.Fatalf("cancel on paid visit: %d %s", r.Status, r.Raw)
	}
	if r := f.move(f.e.Kitchen, id, v, "rejected", "late"); r.Code() != "VISIT_STATE_CONFLICT" {
		t.Fatalf("reject on paid visit: %d", r.Status)
	}
	if r := f.move(f.e.Kitchen, id, v, "accepted"); r.Status != 200 {
		t.Fatalf("progress on paid visit should still work: %d", r.Status)
	}
}

// TestAssistanceCoalescesAndAcknowledges (ORD-006, ORD-A6).
func TestAssistanceCoalescesAndAcknowledges(t *testing.T) {
	f := setup(t)
	a, b := f.e.Diner(f.token), f.e.Diner(f.token)
	path := "/api/v1/visits/" + f.visit + "/assistance"
	first := a.Send(path, "", map[string]any{"topic": "allergy", "note": "peanut allergy"})
	dup := b.Send(path, "", map[string]any{"topic": "allergy"})
	if first.Status != 201 || dup.Status != 200 || dup.Str("id") != first.Str("id") || first.Str("state") != "open" {
		t.Fatalf("coalesce: %d %s / %d %s", first.Status, first.Raw, dup.Status, dup.Raw)
	}
	if r := a.Send(path, "", map[string]any{"topic": "checkout"}); r.Status != 201 {
		t.Fatalf("second topic: %d", r.Status)
	}
	if r := a.Send(path, "", map[string]any{"topic": "refund"}); r.Status != 422 {
		t.Fatalf("bad topic: %d", r.Status)
	}
	board := f.e.StaffGet(f.e.Host, "/api/v1/branches/"+f.e.Branch+"/assistance")
	if board.Len("items") != 2 || board.Str("items", 0, "table_label") != "T1" {
		t.Fatalf("board: %s", board.Raw)
	}
	if r := f.e.StaffGet(f.e.Kitchen, "/api/v1/branches/"+f.e.Branch+"/assistance"); r.Status != 403 {
		t.Fatalf("kitchen assistance board: %d", r.Status)
	}
	id := first.Str("id")
	ack := f.e.StaffSend("POST", f.e.Host, "/api/v1/assistance/"+id+"/transition", map[string]any{"expected_version": 1, "to_state": "acknowledged"})
	if ack.Status != 200 || ack.Str("state") != "acknowledged" || ack.Get("acknowledged_at") == nil {
		t.Fatalf("acknowledge: %d %s", ack.Status, ack.Raw)
	}
	view := a.Get(path)
	if view.Status != 200 || !containsState(view, id, "acknowledged") {
		t.Fatalf("guest view: %s", view.Raw)
	}
	if r := f.e.StaffSend("POST", f.e.Host, "/api/v1/assistance/"+id+"/transition", map[string]any{"expected_version": 2, "to_state": "acknowledged"}); r.Status != 409 {
		t.Fatalf("acknowledge twice: %d", r.Status)
	}
	if r := f.e.StaffSend("POST", f.e.Host, "/api/v1/assistance/"+id+"/transition", map[string]any{"expected_version": 2, "to_state": "resolved"}); r.Status != 200 {
		t.Fatalf("resolve: %d", r.Status)
	}
	if r := a.Send(path, "", map[string]any{"topic": "allergy"}); r.Status != 201 {
		t.Fatalf("new allergy request after resolution: %d", r.Status)
	}
}

func containsState(r testenv.Resp, id, state string) bool {
	for i := range r.Len("items") {
		if r.Str("items", i, "id") == id {
			return r.Str("items", i, "state") == state
		}
	}
	return false
}

// TestKitchenBoardPagination: bounded pages in submission order.
func TestKitchenBoardPagination(t *testing.T) {
	f := setup(t)
	for range 7 {
		f.placeLine()
	}
	seen := map[string]bool{}
	cursor := ""
	for page := 0; page < 10; page++ {
		path := fmt.Sprintf("/api/v1/branches/%s/kitchen-lines?limit=3", f.e.Branch)
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		r := f.e.StaffGet(f.e.Kitchen, path)
		for i := range r.Len("items") {
			seen[r.Str("items", i, "id")] = true
		}
		cursor = r.Str("next_cursor")
		if cursor == "" {
			break
		}
	}
	if len(seen) != 7 {
		t.Fatalf("paged %d lines, want 7", len(seen))
	}
}
