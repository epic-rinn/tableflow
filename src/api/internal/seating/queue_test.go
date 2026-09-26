package seating_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/epic-rinn/tableflow/src/api/internal/access"
	"github.com/epic-rinn/tableflow/src/api/internal/identity"
)

// TestQueueJoinIdempotent (QUE-A1): parallel retries with one key and body
// create one ticket and one tracking capability; all get the same result.
func TestQueueJoinIdempotent(t *testing.T) {
	e := newEnv(t)
	g := e.newGuest()
	key := newKey()
	const n = 6
	results := make([]resp, n)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() { <-start; results[i] = g.join(key, 2) })
	}
	close(start)
	wg.Wait()
	replays := 0
	for i, r := range results {
		if r.status != 201 || r.str("ticket", "id") != results[0].str("ticket", "id") || r.str("tracking", "token") != results[0].str("tracking", "token") {
			t.Fatalf("result %d differs: %d %s", i, r.status, r.raw)
		}
		if r.header.Get("Idempotency-Replayed") == "true" {
			replays++
		}
	}
	if replays != n-1 {
		t.Fatalf("replays = %d", replays)
	}
	if got := e.count("SELECT count(*) FROM queue_tickets"); got != 1 {
		t.Fatalf("tickets = %d", got)
	}
	if got := e.count("SELECT count(*) FROM capabilities WHERE kind = 'queue'"); got != 1 {
		t.Fatalf("capabilities = %d", got)
	}
	if r := g.join(key, 3); r.status != 409 || r.code() != "IDEMPOTENCY_CONFLICT" {
		t.Fatalf("same key, different body: %d %s", r.status, r.raw)
	}
	if r := g.join("", 2); r.status != 400 || r.code() != "IDEMPOTENCY_KEY_REQUIRED" {
		t.Fatalf("missing key: %d %s", r.status, r.raw)
	}
	// Another phone reusing the key is a different scope: a separate ticket.
	if r := e.newGuest().join(key, 2); r.status != 201 || r.str("ticket", "id") == results[0].str("ticket", "id") {
		t.Fatalf("key leaked across sessions: %s", r.raw)
	}
}

// TestGroupPositionIndependentOfOtherGroups (QUE-A2). [QUE-002]
func TestGroupPositionIndependentOfOtherGroups(t *testing.T) {
	e := newEnv(t)
	two := e.table("T2", 2)
	a, aTok := e.joinTicket(2)
	big, bigTok := e.joinTicket(4)
	b, _ := e.joinTicket(2)
	big2, _ := e.joinTicket(3)

	ahead := func(id string) int {
		r := e.staffGet(e.host, "/api/v1/queue-tickets/"+id)
		if r.body["parties_ahead"] == nil {
			return -1
		}
		return r.num("parties_ahead")
	}
	for id, want := range map[string]int{a: 0, b: 1, big: 0, big2: 1} {
		if got := ahead(id); got != want {
			t.Errorf("parties_ahead(%s) = %d, want %d", id, got, want)
		}
	}
	// The two-seat table goes to the oldest 1–2 party without any override,
	// even though the four-person party joined earlier.
	if r := e.call(e.host, a, two); r.status != 200 || r.str("state") != "called" {
		t.Fatalf("call oldest 1–2: %d %s", r.status, r.raw)
	}
	if got := ahead(big2); got != 1 {
		t.Fatalf("3–4 group position moved: %d", got)
	}
	if got := ahead(b); got != 0 {
		t.Fatalf("b should now be first waiting in 1–2: %d", got)
	}
	// Guests see the group label and no one else's details.
	g := e.newGuest()
	g.exchange(bigTok, access.KindQueue)
	r := g.get("/api/v1/queue-tickets/" + big)
	if r.status != 200 || r.str("seating_group", "label") != "3–4" || r.num("parties_ahead") != 0 {
		t.Fatalf("tracking: %d %s", r.status, r.raw)
	}
	_ = aTok
}

// TestJoinOrderSurvivesDailyRenumbering (QUE-A4): a party waiting since
// "yesterday" stays ahead of today's arrivals even when display numbers reset.
func TestJoinOrderSurvivesDailyRenumbering(t *testing.T) {
	e := newEnv(t)
	table := e.table("T4", 4)
	old, _ := e.joinTicket(2)
	e.exec("UPDATE queue_tickets SET business_date = business_date - 1 WHERE id = $1", old)
	e.exec("UPDATE queue_counters SET business_date = business_date - 1")
	newer, _ := e.joinTicket(2)

	on := e.staffGet(e.host, "/api/v1/queue-tickets/"+old)
	nn := e.staffGet(e.host, "/api/v1/queue-tickets/"+newer)
	if on.num("display_number") != 1 || nn.num("display_number") != 1 {
		t.Fatalf("display numbers should both be 1: %d %d", on.num("display_number"), nn.num("display_number"))
	}
	if nn.num("parties_ahead") != 1 {
		t.Fatalf("today's party should see yesterday's ahead: %s", nn.raw)
	}
	board := e.staffGet(e.host, "/api/v1/branches/"+e.branch+"/queue-tickets")
	items := board.body["items"].([]any)
	if len(items) != 2 || items[0].(map[string]any)["id"] != old {
		t.Fatalf("board order: %s", board.raw)
	}
	if r := e.call(e.host, newer, table); r.status != 409 || r.code() != "BYPASS_REQUIRES_OVERRIDE" {
		t.Fatalf("calling the newer party must be a bypass: %d %s", r.status, r.raw)
	}
}

// TestTrackingShowsOnlyOwnTicket: a guest reads only its own ticket. [QUE-002]
func TestTrackingShowsOnlyOwnTicket(t *testing.T) {
	e := newEnv(t)
	mine, tok := e.joinTicket(2, "high_chair")
	theirs, _ := e.joinTicket(3, "accessible")
	g := e.newGuest()
	if r := g.exchange(tok, access.KindQueue); r.status != 201 {
		t.Fatalf("exchange: %d", r.status)
	}
	r := g.get("/api/v1/queue-tickets/" + mine)
	if r.status != 200 || r.header.Get("Cache-Control") != "private, no-store" {
		t.Fatalf("own ticket: %d %s", r.status, r.raw)
	}
	if other := g.get("/api/v1/queue-tickets/" + theirs); other.status != 404 {
		t.Fatalf("other ticket visible: %d %s", other.status, other.raw)
	}
	if board := g.get("/api/v1/branches/" + e.branch + "/queue-tickets"); board.status != 401 {
		t.Fatalf("guest reached the board: %d", board.status)
	}
	if r := e.do("GET", "/api/v1/queue-tickets/"+mine, "", "", "", nil); r.status != 401 {
		t.Fatalf("anonymous read: %d", r.status)
	}
}

// TestQueueBranchIsolation: staff of another branch see nothing.
func TestQueueBranchIsolation(t *testing.T) {
	e := newEnv(t)
	table := e.table("T2", 2)
	tk, _ := e.joinTicket(2)
	var other, otherMgr string
	ctx := context.Background()
	if err := e.pool.QueryRow(ctx, "INSERT INTO branches (name) VALUES ('Other') RETURNING id").Scan(&other); err != nil {
		t.Fatal(err)
	}
	if err := e.pool.QueryRow(ctx, `INSERT INTO staff_accounts (branch_id, email, display_name, status, password_hash)
		SELECT $1, 'other@example.com', 'O', 'active', password_hash FROM staff_accounts WHERE email = 'manager@example.com' RETURNING id`, other).Scan(&otherMgr); err != nil {
		t.Fatal(err)
	}
	e.exec("INSERT INTO staff_roles VALUES ($1, 'manager'), ($1, 'host')", otherMgr)
	x := e.login("other@example.com")
	checks := []resp{
		e.staffGet(x, "/api/v1/queue-tickets/"+tk),
		e.staffGet(x, "/api/v1/branches/"+e.branch+"/queue-tickets"),
		e.staffGet(x, "/api/v1/branches/"+e.branch+"/tables"),
		e.staffPost(x, "/api/v1/queue-tickets/"+tk+"/call", map[string]any{"table_id": table, "expected_version": 1}),
		e.staffPost(x, "/api/v1/queue-tickets/"+tk+"/cancel", map[string]any{"expected_version": 1}),
		e.staffPost(x, "/api/v1/visits", e.seatBody(table, nil, 2)),
		e.do("POST", "/api/v1/branches/"+e.branch+"/queue-tickets", staffCookie(x), adminOrigin, newKey(), map[string]any{"party_size": 2, "needs": []string{}}),
	}
	for i, r := range checks {
		if r.status != 404 {
			t.Errorf("check %d: %d %s", i, r.status, r.raw)
		}
	}
	if got := e.count("SELECT count(*) FROM table_claims"); got != 0 {
		t.Fatal("cross-branch command created a claim")
	}
}

// TestTableConfiguration (OPS-001 tables subset).
func TestTableConfiguration(t *testing.T) {
	e := newEnv(t)
	id := e.table("A1", 4, "accessible")
	if r := e.do("POST", "/api/v1/branches/"+e.branch+"/tables", staffCookie(e.host), adminOrigin, "",
		map[string]any{"label": "X", "capacity": 2, "needs": []string{}}); r.status != 403 {
		t.Fatalf("host created a table: %d", r.status)
	}
	if r := e.do("POST", "/api/v1/branches/"+e.branch+"/tables", staffCookie(e.manager), adminOrigin, "",
		map[string]any{"label": "A1", "capacity": 2, "needs": []string{}}); r.code() != "LABEL_TAKEN" {
		t.Fatalf("duplicate label: %d %s", r.status, r.raw)
	}
	if r := e.do("POST", "/api/v1/branches/"+e.branch+"/tables", staffCookie(e.manager), adminOrigin, "",
		map[string]any{"label": "Z", "capacity": 0, "needs": []string{"pool"}}); r.status != 422 {
		t.Fatalf("invalid table: %d", r.status)
	}
	patch := func(v int, body map[string]any) resp {
		body["expected_version"] = v
		return e.do("PATCH", "/api/v1/tables/"+id, staffCookie(e.manager), adminOrigin, "", body)
	}
	r := patch(1, map[string]any{"label": "A1", "capacity": 6, "needs": []string{"accessible", "high_chair"}, "active": true})
	if r.status != 200 || r.num("capacity") != 6 || r.num("version") != 2 {
		t.Fatalf("update: %d %s", r.status, r.raw)
	}
	if r := patch(1, map[string]any{"label": "A1", "capacity": 6, "needs": []string{}, "active": true}); r.code() != "VERSION_CONFLICT" {
		t.Fatalf("stale update: %d", r.status)
	}
	e.seatWalkIn(id, 5)
	if r := patch(e.tableVersion(id), map[string]any{"label": "A1", "capacity": 6, "needs": []string{"accessible", "high_chair"}, "active": false}); r.code() != "TABLE_UNAVAILABLE" {
		t.Fatalf("deactivated an occupied table: %d %s", r.status, r.raw)
	}
	if r := patch(e.tableVersion(id), map[string]any{"label": "A1", "capacity": 4, "needs": []string{"accessible", "high_chair"}, "active": true}); r.code() != "TABLE_UNAVAILABLE" {
		t.Fatalf("shrank an occupied table: %d %s", r.status, r.raw)
	}
	board := e.staffGet(e.host, "/api/v1/branches/"+e.branch+"/tables")
	item := board.body["items"].([]any)[0].(map[string]any)
	if item["state"] != "occupied" || item["claim"].(map[string]any)["kind"] != "visit" {
		t.Fatalf("board: %s", board.raw)
	}
}

// TestSeatingGroupValidation.
func TestSeatingGroupValidation(t *testing.T) {
	e := newEnv(t)
	put := func(cookie string, groups []map[string]any) resp {
		return e.do("PUT", "/api/v1/branches/"+e.branch+"/seating-groups", staffCookie(cookie), adminOrigin, "", map[string]any{"groups": groups})
	}
	if r := put(e.manager, []map[string]any{{"label": "a", "min_party": 1, "max_party": 2}, {"label": "b", "min_party": 4, "max_party": 6}}); r.status != 422 {
		t.Fatalf("gap accepted: %d", r.status)
	}
	if r := put(e.host, []map[string]any{{"label": "all", "min_party": 1, "max_party": 8}}); r.status != 403 {
		t.Fatalf("host changed groups: %d", r.status)
	}
	if r := e.newGuest().join(newKey(), 7); r.status != 422 || r.code() != "PARTY_NEEDS_STAFF" {
		t.Fatalf("guest party of 7: %d %s", r.status, r.raw)
	}
	staffJoin := e.do("POST", "/api/v1/branches/"+e.branch+"/queue-tickets", staffCookie(e.host), adminOrigin, newKey(), map[string]any{"party_size": 7, "needs": []string{}})
	if staffJoin.status != 201 || staffJoin.body["ticket"].(map[string]any)["seating_group"] != nil || staffJoin.str("ticket", "source") != "staff" {
		t.Fatalf("staff join of 7: %d %s", staffJoin.status, staffJoin.raw)
	}
	if r := put(e.manager, []map[string]any{{"label": "all", "min_party": 1, "max_party": 8}}); r.status != 409 || r.code() != "QUEUE_ACTIVE" {
		t.Fatalf("groups replaced with an active queue: %d %s", r.status, r.raw)
	}
	id := staffJoin.str("ticket", "id")
	e.staffPost(e.host, "/api/v1/queue-tickets/"+id+"/cancel", map[string]any{"expected_version": e.ticketVersion(id)})
	r := put(e.manager, []map[string]any{{"label": "small", "min_party": 1, "max_party": 3}, {"label": "large", "min_party": 4, "max_party": 8}})
	if r.status != 200 || len(r.body["groups"].([]any)) != 2 {
		t.Fatalf("replace: %d %s", r.status, r.raw)
	}
	if r := e.newGuest().join(newKey(), 7); r.status != 201 || r.str("ticket", "seating_group", "label") != "large" {
		t.Fatalf("join after replace: %d %s", r.status, r.raw)
	}
}

// TestGuestAndHostCancel (QUE-004 waiting cancellation).
func TestGuestAndHostCancel(t *testing.T) {
	e := newEnv(t)
	mine, tok := e.joinTicket(2)
	other, _ := e.joinTicket(2)
	g := e.newGuest()
	g.exchange(tok, access.KindQueue)
	if r := g.post("/api/v1/queue-tickets/"+other+"/cancel", map[string]any{"expected_version": 1}); r.status != 404 {
		t.Fatalf("guest cancelled another ticket: %d %s", r.status, r.raw)
	}
	if r := g.post("/api/v1/queue-tickets/"+mine+"/cancel", map[string]any{"expected_version": 1}); r.status != 200 || r.str("state") != "cancelled" {
		t.Fatalf("guest cancel: %d %s", r.status, r.raw)
	}
	if r := g.get("/api/v1/queue-tickets/" + mine); r.status != 200 || r.str("state") != "cancelled" || r.body["parties_ahead"] != nil {
		t.Fatalf("tracking after cancel: %d %s", r.status, r.raw)
	}
	if r := g.post("/api/v1/queue-tickets/"+mine+"/cancel", map[string]any{"expected_version": e.ticketVersion(mine)}); r.code() != "TICKET_STATE_CONFLICT" {
		t.Fatalf("second cancel: %d %s", r.status, r.raw)
	}
	if r := e.staffPost(e.host, "/api/v1/queue-tickets/"+other+"/cancel", map[string]any{"expected_version": 1}); r.status != 200 {
		t.Fatalf("host cancel: %d %s", r.status, r.raw)
	}
	// Cancellation from the admin origin with a guest cookie is rejected.
	if r := e.do("POST", "/api/v1/queue-tickets/"+mine+"/cancel", access.GuestCookie+"="+g.sess, adminOrigin, newKey(), map[string]any{"expected_version": 1}); r.status != 403 {
		t.Fatalf("guest via admin origin: %d", r.status)
	}
	if got := e.count("SELECT count(*) FROM capabilities WHERE kind = 'queue' AND expires_at IS NOT NULL"); got != 2 {
		t.Fatalf("terminal tickets should get a tracking expiry: %d", got)
	}
}

// TestJoinRateLimitedPerSession.
func TestJoinRateLimitedPerSession(t *testing.T) {
	e := newEnv(t)
	g := e.newGuest()
	for i := range 5 {
		if r := g.join(newKey(), 2); r.status != 201 {
			t.Fatalf("join %d: %d", i, r.status)
		}
	}
	if r := g.join(newKey(), 2); r.status != 429 {
		t.Fatalf("6th join: %d", r.status)
	}
	_ = fmt.Sprint(identity.RoleHost)
}
