package seating_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/epic-rinn/tableflow/src/api/internal/access"
)

// TestSeatingContractConformance: representative success and error
// responses of the seating operations match OpenAPI.
func TestSeatingContractConformance(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	doc, err := openapi3.NewLoader().LoadFromFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "specs", "api", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatal(err)
	}
	e := newEnv(t)
	check := func(method, path string, r resp) {
		t.Helper()
		ref := doc.Paths.Find(path).GetOperation(method).Responses.Status(r.status)
		if ref == nil {
			t.Fatalf("%s %s: %d undocumented (%s)", method, path, r.status, r.raw)
		}
		var body any
		_ = json.Unmarshal([]byte(r.raw), &body)
		if err := ref.Value.Content.Get("application/json").Schema.Value.VisitJSON(body, openapi3.EnableJSONSchema2020()); err != nil {
			t.Fatalf("%s %s %d: %v\n%s", method, path, r.status, err, r.raw)
		}
	}
	b := "/api/v1/branches/" + e.branch
	check("GET", "/api/v1/branches/{branch_id}/seating-groups", e.staffGet(e.host, b+"/seating-groups"))
	table := e.table("C1", 4, "high_chair")
	two := e.table("C2", 2)
	check("GET", "/api/v1/branches/{branch_id}/tables", e.staffGet(e.host, b+"/tables"))
	g := e.newGuest()
	join := g.join(newKey(), 2, "high_chair")
	check("POST", "/api/v1/branches/{branch_id}/queue-tickets", join)
	check("POST", "/api/v1/branches/{branch_id}/queue-tickets", g.join(newKey(), 60))
	tk := join.str("ticket", "id")
	g.exchange(join.str("tracking", "token"), access.KindQueue)
	check("GET", "/api/v1/queue-tickets/{ticket_id}", g.get("/api/v1/queue-tickets/"+tk))
	check("GET", "/api/v1/branches/{branch_id}/queue-tickets", e.staffGet(e.host, b+"/queue-tickets"))
	check("POST", "/api/v1/queue-tickets/{ticket_id}/call", e.call(e.host, tk, two))
	check("POST", "/api/v1/queue-tickets/{ticket_id}/call", e.call(e.host, tk, table))
	check("GET", "/api/v1/branches/{branch_id}/tables", e.staffGet(e.host, b+"/tables"))
	seat := e.staffPost(e.host, "/api/v1/visits", e.seatBody(table, &tk, 0))
	check("POST", "/api/v1/visits", seat)
	visit := seat.str("visit", "id")
	check("GET", "/api/v1/visits/{visit_id}", e.staffGet(e.host, "/api/v1/visits/"+visit))
	check("POST", "/api/v1/visits/{visit_id}/rotate-access", e.visitPost(visit, "rotate-access", map[string]any{"reason": "test"}))
	check("POST", "/api/v1/visits/{visit_id}/move", e.visitPost(visit, "move", map[string]any{"table_id": two, "expected_table_version": e.tableVersion(two)}))
	check("POST", "/api/v1/visits/{visit_id}/depart", e.visitPost(visit, "depart", map[string]any{}))
	check("POST", "/api/v1/visits/{visit_id}/close-empty", e.visitPost(visit, "close-empty", map[string]any{"reason": "empty"}))
}
