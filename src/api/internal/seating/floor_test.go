package seating_test

import (
	"sync"
	"testing"
)

// parallel runs fns simultaneously behind a barrier on independent
// connections (each HTTP request uses its own pooled connection).
func parallel(fns ...func() resp) []resp {
	out := make([]resp, len(fns))
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i, f := range fns {
		wg.Go(func() { <-start; out[i] = f() })
	}
	close(start)
	wg.Wait()
	return out
}

func oneWinner(t *testing.T, rs []resp, okStatus int) int {
	t.Helper()
	win := -1
	for i, r := range rs {
		switch {
		case r.status == okStatus:
			if win >= 0 {
				t.Fatalf("two winners: %s / %s", rs[win].raw, r.raw)
			}
			win = i
		case r.status == 409:
		default:
			t.Fatalf("unexpected %d %s", r.status, r.raw)
		}
	}
	if win < 0 {
		t.Fatalf("no winner: %v", rs)
	}
	return win
}

// TestConcurrentCallsOneClaim (SEA-A1): two hosts call different parties to
// the same table at the same moment; exactly one hold exists.
func TestConcurrentCallsOneClaim(t *testing.T) {
	for round := range 5 {
		e := newEnv(t)
		table := e.table("T", 4)
		a, _ := e.joinTicket(2)
		b, _ := e.joinTicket(2)
		rs := parallel(
			func() resp { return e.call(e.host, a, table) },
			func() resp { return e.call(e.manager, b, table, "regular guest request") },
		)
		oneWinner(t, rs, 200)
		if got := e.count("SELECT count(*) FROM table_claims WHERE table_id = $1", table); got != 1 {
			t.Fatalf("round %d: claims = %d", round, got)
		}
		if got := e.count("SELECT count(*) FROM queue_tickets WHERE state = 'called'"); got != 1 {
			t.Fatalf("round %d: called tickets = %d", round, got)
		}
	}
}

// TestConcurrentSeatsOneVisit (SEA-A1): two hosts seat walk-ins at one table.
func TestConcurrentSeatsOneVisit(t *testing.T) {
	for round := range 5 {
		e := newEnv(t)
		table := e.table("T", 4)
		body := e.seatBody(table, nil, 2)
		rs := parallel(
			func() resp { return e.staffPost(e.host, "/api/v1/visits", body) },
			func() resp { return e.staffPost(e.host2, "/api/v1/visits", body) },
		)
		oneWinner(t, rs, 201)
		if got := e.count("SELECT count(*) FROM visits"); got != 1 {
			t.Fatalf("round %d: visits = %d", round, got)
		}
		if got := e.count("SELECT count(*) FROM capabilities WHERE kind = 'visit'"); got != 1 {
			t.Fatalf("round %d: dining capabilities = %d", round, got)
		}
	}
}

// TestNoShowVersusSeatOneWinner (QUE-A3): an overdue called party arrives
// while another host marks it a no-show; exactly one outcome commits.
func TestNoShowVersusSeatOneWinner(t *testing.T) {
	for round := range 5 {
		e := newEnv(t)
		table := e.table("T", 4)
		tk, _ := e.joinTicket(2)
		if r := e.call(e.host, tk, table); r.status != 200 {
			t.Fatalf("call: %d %s", r.status, r.raw)
		}
		e.exec("UPDATE queue_tickets SET called_until = now() - interval '1 minute' WHERE id = $1", tk)
		v := e.ticketVersion(tk)
		seat := e.seatBody(table, &tk, 0)
		rs := parallel(
			func() resp { return e.staffPost(e.host, "/api/v1/visits", seat) },
			func() resp {
				return e.staffPost(e.host2, "/api/v1/queue-tickets/"+tk+"/no-show", map[string]any{"expected_version": v, "reason": "did not return"})
			},
		)
		seated, noShow := rs[0].status == 201, rs[1].status == 200
		if seated == noShow {
			t.Fatalf("round %d: seat=%d no-show=%d", round, rs[0].status, rs[1].status)
		}
		var state, tableState string
		e.pool.QueryRow(t.Context(), "SELECT state FROM queue_tickets WHERE id = $1", tk).Scan(&state)
		e.pool.QueryRow(t.Context(), "SELECT state FROM dining_tables WHERE id = $1", table).Scan(&tableState)
		visits := e.count("SELECT count(*) FROM visits")
		if seated && (state != "seated" || tableState != "occupied" || visits != 1) {
			t.Fatalf("round %d seated: ticket=%s table=%s visits=%d", round, state, tableState, visits)
		}
		if noShow && (state != "no_show" || tableState != "available" || visits != 0 || e.count("SELECT count(*) FROM table_claims") != 0) {
			t.Fatalf("round %d no-show: ticket=%s table=%s visits=%d", round, state, tableState, visits)
		}
	}
}

// TestSeatRetryReturnsOriginal: repeating a successful seat returns the same
// visit and dining token without creating anything.
func TestSeatRetryReturnsOriginal(t *testing.T) {
	e := newEnv(t)
	table := e.table("T", 4)
	body := e.seatBody(table, nil, 3)
	key := newKey()
	first := e.do("POST", "/api/v1/visits", staffCookie(e.host), adminOrigin, key, body)
	again := e.do("POST", "/api/v1/visits", staffCookie(e.host), adminOrigin, key, body)
	if first.status != 201 || again.status != 201 || again.header.Get("Idempotency-Replayed") != "true" {
		t.Fatalf("seat/retry: %d %d", first.status, again.status)
	}
	if first.str("visit", "id") != again.str("visit", "id") || first.str("dining", "token") != again.str("dining", "token") {
		t.Fatalf("retry returned a different visit: %s / %s", first.raw, again.raw)
	}
	if got := e.count("SELECT count(*) FROM visits"); got != 1 {
		t.Fatalf("visits = %d", got)
	}
	if got := e.count("SELECT count(*) FROM idempotency_requests WHERE position(convert_to($1, 'UTF8') IN response) > 0", first.str("dining", "token")); got != 0 {
		t.Fatal("dining token stored in plaintext")
	}
}

// TestOverdueHoldPersists (QUE-004): a passed deadline is flagged but the
// hold remains until staff act.
func TestOverdueHoldPersists(t *testing.T) {
	e := newEnv(t)
	table := e.table("T", 4)
	tk, _ := e.joinTicket(2)
	e.call(e.host, tk, table)
	e.exec("UPDATE queue_tickets SET called_until = now() - interval '10 minutes' WHERE id = $1", tk)
	board := e.staffGet(e.host, "/api/v1/branches/"+e.branch+"/queue-tickets?state=called")
	item := board.body["items"].([]any)[0].(map[string]any)
	if item["overdue"] != true || item["called_table_label"] != "T" {
		t.Fatalf("board: %s", board.raw)
	}
	tables := e.staffGet(e.host, "/api/v1/branches/"+e.branch+"/tables")
	if st := tables.body["items"].([]any)[0].(map[string]any)["state"]; st != "held" {
		t.Fatalf("table state %v", st)
	}
	if r := e.staffPost(e.host, "/api/v1/visits", e.seatBody(table, &tk, 0)); r.status != 201 {
		t.Fatalf("late arrival could not be seated: %d %s", r.status, r.raw)
	}
}

// TestBypassRequiresManagerReason (QUE-003, SEA-001).
func TestBypassRequiresManagerReason(t *testing.T) {
	e := newEnv(t)
	table := e.table("T", 4)
	older, _ := e.joinTicket(2)
	newer, _ := e.joinTicket(2)
	if r := e.call(e.host, newer, table); r.code() != "BYPASS_REQUIRES_OVERRIDE" {
		t.Fatalf("host bypass without reason: %d %s", r.status, r.raw)
	}
	if r := e.call(e.host, newer, table, "friend of owner"); r.status != 403 {
		t.Fatalf("host bypass with reason: %d", r.status)
	}
	if r := e.staffPost(e.host, "/api/v1/visits", e.seatBody(table, nil, 2)); r.code() != "BYPASS_REQUIRES_OVERRIDE" {
		t.Fatalf("walk-in ahead of the queue: %d %s", r.status, r.raw)
	}
	if r := e.call(e.manager, newer, table, "booked by phone earlier"); r.status != 200 {
		t.Fatalf("manager override: %d %s", r.status, r.raw)
	}
	if got := e.count("SELECT count(*) FROM audit_events WHERE action = 'queue.bypass' AND reason = 'booked by phone earlier'"); got != 1 {
		t.Fatalf("bypass audit rows = %d", got)
	}
	_ = older
}

// TestIncompatibleTableRejected.
func TestIncompatibleTableRejected(t *testing.T) {
	e := newEnv(t)
	small := e.table("S", 2)
	plain := e.table("P", 6)
	four, _ := e.joinTicket(4)
	chair, _ := e.joinTicket(2, "high_chair")
	if r := e.call(e.host, four, small); r.code() != "TABLE_INCOMPATIBLE" {
		t.Fatalf("party of 4 at a 2-top: %d %s", r.status, r.raw)
	}
	if r := e.call(e.manager, chair, plain, "x"); r.code() != "TABLE_INCOMPATIBLE" {
		t.Fatalf("high chair at a table without one: %d %s", r.status, r.raw)
	}
	if r := e.staffPost(e.manager, "/api/v1/visits", map[string]any{"branch_id": e.branch, "table_id": small,
		"expected_table_version": e.tableVersion(small), "party_size": 3, "needs": []string{}}); r.code() != "TABLE_INCOMPATIBLE" {
		t.Fatalf("walk-in too large: %d %s", r.status, r.raw)
	}
}

// TestCancelCalledReleasesHold (QUE-004).
func TestCancelCalledReleasesHold(t *testing.T) {
	e := newEnv(t)
	table := e.table("T", 4)
	tk, _ := e.joinTicket(2)
	e.call(e.host, tk, table)
	if r := e.staffPost(e.host, "/api/v1/queue-tickets/"+tk+"/cancel", map[string]any{"expected_version": e.ticketVersion(tk)}); r.status != 200 {
		t.Fatalf("cancel called: %d %s", r.status, r.raw)
	}
	if got := e.count("SELECT count(*) FROM table_claims"); got != 0 {
		t.Fatal("hold not released")
	}
	if r := e.staffPost(e.host, "/api/v1/visits", e.seatBody(table, nil, 2)); r.status != 201 {
		t.Fatalf("table not available after cancel: %d %s", r.status, r.raw)
	}
}
