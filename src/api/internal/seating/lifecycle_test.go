package seating_test

import (
	"testing"

	"github.com/epic-rinn/tableflow/src/api/internal/access"
)

func (e *env) visitPost(visit, action string, body map[string]any) resp {
	if _, ok := body["expected_version"]; !ok {
		body["expected_version"] = e.visitVersion(visit)
	}
	return e.staffPost(e.host, "/api/v1/visits/"+visit+"/"+action, body)
}

func (e *env) tableState(id string) string {
	var s string
	e.pool.QueryRow(e.t.Context(), "SELECT state FROM dining_tables WHERE id = $1", id).Scan(&s)
	return s
}

// TestMoveKeepsAccessAndCleansOldTable (SEA-A2).
func TestMoveKeepsAccessAndCleansOldTable(t *testing.T) {
	e := newEnv(t)
	x := e.table("X", 4)
	y := e.table("Y", 4)
	visit, token := e.seatWalkIn(x, 3)
	g := e.newGuest()
	g.exchange(token, access.KindVisit)
	r := e.visitPost(visit, "move", map[string]any{"table_id": y, "expected_table_version": e.tableVersion(y)})
	if r.status != 200 || r.str("table", "label") != "Y" {
		t.Fatalf("move: %d %s", r.status, r.raw)
	}
	if got := g.get("/api/v1/visits/" + visit); got.status != 200 || got.str("table", "label") != "Y" {
		t.Fatalf("original QR after move: %d %s", got.status, got.raw)
	}
	if e.tableState(x) != "cleaning" || e.tableState(y) != "occupied" {
		t.Fatalf("states x=%s y=%s", e.tableState(x), e.tableState(y))
	}
	if got := e.count("SELECT count(*) FROM table_claims WHERE table_id = $1 AND visit_id = $2", y, visit); got != 1 {
		t.Fatal("claim did not move")
	}
	if r := e.staffPost(e.host, "/api/v1/tables/"+x+"/ready", map[string]any{"expected_version": e.tableVersion(x)}); r.status != 200 || r.str("state") != "available" {
		t.Fatalf("ready: %d %s", r.status, r.raw)
	}
	if r := e.visitPost(visit, "move", map[string]any{"table_id": y, "expected_table_version": e.tableVersion(y)}); r.status != 422 {
		t.Fatalf("move to own table: %d", r.status)
	}
}

// TestRotateAccessInvalidatesOldQR (ACC-A2).
func TestRotateAccessInvalidatesOldQR(t *testing.T) {
	e := newEnv(t)
	x := e.table("X", 4)
	visit, token := e.seatWalkIn(x, 2)
	g := e.newGuest()
	g.exchange(token, access.KindVisit)
	r := e.visitPost(visit, "rotate-access", map[string]any{"reason": "QR photographed by passer-by"})
	if r.status != 200 || r.str("dining", "token") == "" || r.str("dining", "token") == token {
		t.Fatalf("rotate: %d %s", r.status, r.raw)
	}
	if got := g.get("/api/v1/visits/" + visit); got.status != 401 {
		t.Fatalf("old session after rotation: %d", got.status)
	}
	if ex := e.newGuest().exchange(token, access.KindVisit); ex.status != 422 {
		t.Fatalf("old QR after rotation: %d", ex.status)
	}
	for range 2 {
		n := e.newGuest()
		n.exchange(r.str("dining", "token"), access.KindVisit)
		if got := n.get("/api/v1/visits/" + visit); got.status != 200 {
			t.Fatalf("new QR: %d", got.status)
		}
	}
	if got := e.count("SELECT count(*) FROM audit_events WHERE action = 'visit.access_rotated'"); got != 1 {
		t.Fatalf("audit rows %d", got)
	}
}

// TestMoveVersusDepartOneWinner: concurrent move and departure of a paid
// visit; exactly one commits and the claim invariants hold.
func TestMoveVersusDepartOneWinner(t *testing.T) {
	for round := range 5 {
		e := newEnv(t)
		x := e.table("X", 4)
		y := e.table("Y", 4)
		visit, _ := e.seatWalkIn(x, 2)
		e.exec("UPDATE visits SET state = 'paid', paid_at = now(), version = version + 1 WHERE id = $1", visit) // fixture until MVP-12
		v, yv := e.visitVersion(visit), e.tableVersion(y)
		rs := parallel(
			func() resp {
				return e.staffPost(e.host, "/api/v1/visits/"+visit+"/move", map[string]any{"table_id": y, "expected_version": v, "expected_table_version": yv})
			},
			func() resp {
				return e.staffPost(e.host2, "/api/v1/visits/"+visit+"/depart", map[string]any{"expected_version": v})
			},
		)
		win := oneWinner(t, rs, 200)
		claims := e.count("SELECT count(*) FROM table_claims")
		if win == 0 && (claims != 1 || e.tableState(y) != "occupied" || e.tableState(x) != "cleaning") {
			t.Fatalf("round %d move won: claims=%d x=%s y=%s", round, claims, e.tableState(x), e.tableState(y))
		}
		if win == 1 && (claims != 0 || e.tableState(x) != "cleaning" || e.tableState(y) != "available") {
			t.Fatalf("round %d depart won: claims=%d x=%s y=%s", round, claims, e.tableState(x), e.tableState(y))
		}
	}
}

// TestPaidVisitKeepsTable (SEA-A3, SEA-004).
func TestPaidVisitKeepsTable(t *testing.T) {
	e := newEnv(t)
	x := e.table("X", 4)
	visit, token := e.seatWalkIn(x, 2)
	if r := e.visitPost(visit, "depart", map[string]any{}); r.code() != "VISIT_STATE_CONFLICT" {
		t.Fatalf("depart unpaid: %d %s", r.status, r.raw)
	}
	e.exec("UPDATE visits SET state = 'paid', paid_at = now(), version = version + 1 WHERE id = $1", visit)
	tk, _ := e.joinTicket(2)
	if r := e.call(e.host, tk, x); r.code() != "TABLE_UNAVAILABLE" {
		t.Fatalf("called onto a paid table: %d %s", r.status, r.raw)
	}
	if r := e.staffPost(e.manager, "/api/v1/visits", e.seatBody(x, nil, 2)); r.code() != "TABLE_UNAVAILABLE" && r.code() != "BYPASS_REQUIRES_OVERRIDE" {
		t.Fatalf("seated onto a paid table: %d %s", r.status, r.raw)
	}
	if r := e.staffPost(e.host, "/api/v1/tables/"+x+"/ready", map[string]any{"expected_version": e.tableVersion(x)}); r.code() != "TABLE_UNAVAILABLE" {
		t.Fatalf("ready while occupied: %d", r.status)
	}
	if r := e.visitPost(visit, "depart", map[string]any{}); r.status != 200 || r.str("state") != "departed" {
		t.Fatalf("depart: %d %s", r.status, r.raw)
	}
	if ex := e.newGuest().exchange(token, access.KindVisit); ex.status != 422 {
		t.Fatalf("dining QR valid after departure: %d", ex.status)
	}
	if r := e.staffPost(e.host, "/api/v1/tables/"+x+"/ready", map[string]any{"expected_version": e.tableVersion(x)}); r.status != 200 {
		t.Fatalf("ready after departure: %d %s", r.status, r.raw)
	}
	if r := e.call(e.host, tk, x); r.status != 200 {
		t.Fatalf("call after cleaning: %d %s", r.status, r.raw)
	}
}

// TestCloseEmptyAndReady (SEA-004).
func TestCloseEmptyAndReady(t *testing.T) {
	e := newEnv(t)
	x := e.table("X", 4)
	visit, _ := e.seatWalkIn(x, 2)
	if r := e.visitPost(visit, "close-empty", map[string]any{"reason": ""}); r.status != 422 {
		t.Fatalf("close without reason: %d", r.status)
	}
	if r := e.visitPost(visit, "close-empty", map[string]any{"reason": "party left before ordering"}); r.status != 200 || r.str("state") != "closed" {
		t.Fatalf("close-empty: %d %s", r.status, r.raw)
	}
	if e.tableState(x) != "cleaning" {
		t.Fatalf("table %s", e.tableState(x))
	}
	if got := e.count("SELECT count(*) FROM audit_events WHERE action = 'visit.closed_empty' AND reason = 'party left before ordering'"); got != 1 {
		t.Fatal("close not audited")
	}
	if r := e.visitPost(visit, "close-empty", map[string]any{"reason": "again"}); r.code() != "VISIT_STATE_CONFLICT" {
		t.Fatalf("close twice: %d", r.status)
	}
}

// TestRotateAccessOnlyBeforePayment: a new dining QR cannot restore guest
// access to a paid visit (BIL-008).
func TestRotateAccessOnlyBeforePayment(t *testing.T) {
	e := newEnv(t)
	visit, _ := e.seatWalkIn(e.table("P", 2), 2)
	e.exec("UPDATE visits SET state = 'paid', paid_at = now() WHERE id = $1", visit)
	if r := e.visitPost(visit, "rotate-access", map[string]any{"reason": "guest lost the QR"}); r.status != 409 {
		t.Fatalf("rotate after payment: %d %s", r.status, r.raw)
	}
}
