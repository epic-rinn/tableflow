package ordering_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/epic-rinn/tableflow/src/api/internal/testenv"
)

// TestOrderingContractConformance: menu, order, kitchen and assistance
// responses match OpenAPI.
func TestOrderingContractConformance(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	doc, err := openapi3.NewLoader().LoadFromFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "specs", "api", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatal(err)
	}
	check := func(method, path string, r testenv.Resp) {
		t.Helper()
		ref := doc.Paths.Find(path).GetOperation(method).Responses.Status(r.Status)
		if ref == nil {
			t.Fatalf("%s %s: %d undocumented (%s)", method, path, r.Status, r.Raw)
		}
		var body any
		_ = json.Unmarshal([]byte(r.Raw), &body)
		if err := ref.Value.Content.Get("application/json").Schema.Value.VisitJSON(body, openapi3.EnableJSONSchema2020()); err != nil {
			t.Fatalf("%s %s %d: %v\n%s", method, path, r.Status, err, r.Raw)
		}
	}
	f := setup(t)
	e := f.e
	check("GET", "/api/v1/branches/{branch_id}/menu", e.Do("GET", "/api/v1/branches/"+e.Branch+"/menu", "", "", "", nil))
	check("PUT", "/api/v1/branches/{branch_id}/menu", e.Do("PUT", "/api/v1/branches/"+e.Branch+"/menu", testenv.StaffCookie(e.Manager), testenv.AdminOrigin, "",
		map[string]any{"expected_revision": 1, "categories": []any{}}))
	v := testenv.Scalar[int](e, "SELECT version FROM menu_items WHERE id = $1", f.pad)
	check("PATCH", "/api/v1/menu-items/{item_id}/availability", e.Do("PATCH", "/api/v1/menu-items/"+f.pad+"/availability", testenv.StaffCookie(e.Manager), testenv.AdminOrigin, "",
		map[string]any{"expected_version": v, "sold_out": false}))
	g := e.Diner(f.token)
	cur := testenv.Scalar[int](e, "SELECT revision FROM menus WHERE branch_id = $1", e.Branch)
	o := g.Send(f.ordersPath(), "", map[string]any{"menu_revision": cur, "lines": []map[string]any{line(f.pad, 1, f.shrimp, f.egg)}})
	check("POST", "/api/v1/visits/{visit_id}/orders", o)
	check("POST", "/api/v1/visits/{visit_id}/orders", g.Send(f.ordersPath(), "", map[string]any{"menu_revision": 1, "lines": []map[string]any{line(f.tea, 1)}}))
	check("GET", "/api/v1/visits/{visit_id}/orders", g.Get(f.ordersPath()))
	check("GET", "/api/v1/branches/{branch_id}/kitchen-lines", e.StaffGet(e.Kitchen, "/api/v1/branches/"+e.Branch+"/kitchen-lines"))
	check("POST", "/api/v1/order-lines/{line_id}/transition", f.move(e.Kitchen, o.Str("lines", 0, "id"), 1, "accepted"))
	a := g.Send("/api/v1/visits/"+f.visit+"/assistance", "", map[string]any{"topic": "help"})
	check("POST", "/api/v1/visits/{visit_id}/assistance", a)
	check("GET", "/api/v1/visits/{visit_id}/assistance", g.Get("/api/v1/visits/"+f.visit+"/assistance"))
	check("GET", "/api/v1/branches/{branch_id}/assistance", e.StaffGet(e.Host, "/api/v1/branches/"+e.Branch+"/assistance"))
	check("POST", "/api/v1/assistance/{request_id}/transition", e.StaffSend("POST", e.Host, "/api/v1/assistance/"+a.Str("id")+"/transition",
		map[string]any{"expected_version": 1, "to_state": "resolved"}))
}
