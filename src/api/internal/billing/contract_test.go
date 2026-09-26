package billing_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/epic-rinn/tableflow/src/api/internal/testenv"
)

type checker func(method, path string, r testenv.Resp)

func contract(t *testing.T) checker {
	t.Helper()
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
	return check
}

// TestBillingContractConformance: policy, bill, settlement, receipt and
// refund responses (including conflict bodies) match OpenAPI.
func TestBillingContractConformance(t *testing.T) {
	check := contract(t)
	f := setup(t)
	e := f.e
	pol := "/api/v1/branches/" + e.Branch + "/charge-policy"
	check("GET", "/api/v1/branches/{branch_id}/charge-policy", e.StaffGet(e.Cashier, pol))
	check("PUT", "/api/v1/branches/{branch_id}/charge-policy", e.StaffSend("PUT", e.Manager, pol,
		map[string]any{"expected_version": 0, "tax_mode": "exclusive", "tax_bp": 700, "service_bp": 1000}))
	id, v := f.order(f.tea, 1)
	check("GET", "/api/v1/visits/{visit_id}/bill", f.e.Diner(f.token).Get(f.path("/bill")))
	check("POST", "/api/v1/bills/resolve", e.StaffSend("POST", e.Cashier, "/api/v1/bills/resolve", map[string]any{"dining_token": f.token}))
	bv := f.bill(e.Cashier).Num("bill_version")
	check("POST", "/api/v1/visits/{visit_id}/settlement/begin", f.begin(e.Cashier, bv)) // UNRESOLVED_LINES
	f.serve(id, v)
	check("POST", "/api/v1/visits/{visit_id}/settlement/begin", f.begin(e.Cashier, bv-1)) // BILL_VERSION_CONFLICT
	s := f.begin(e.Cashier, bv)
	check("POST", "/api/v1/visits/{visit_id}/settlement/begin", s)
	r := e.StaffSend("POST", e.Cashier, f.path("/settlement/reopen"), map[string]any{"expected_version": s.Num("bill_version"), "reason": "more"})
	check("POST", "/api/v1/visits/{visit_id}/settlement/reopen", r)
	s = f.begin(e.Cashier, r.Num("bill_version"))
	c := f.confirm(e.Cashier, "", s.Num("bill_version"), s.Num("total_satang"))
	check("POST", "/api/v1/visits/{visit_id}/settlement/confirm", c)
	check("POST", "/api/v1/visits/{visit_id}/settlement/confirm", f.confirm(e.Cashier, "", s.Num("bill_version"), s.Num("total_satang"))) // ALREADY_PAID
	check("GET", "/api/v1/visits/{visit_id}/bill", f.bill(e.Cashier))
	check("GET", "/api/v1/branches/{branch_id}/settlements", e.StaffGet(e.Cashier, "/api/v1/branches/"+e.Branch+"/settlements"))
	check("GET", "/api/v1/settlements/{settlement_id}", e.StaffGet(e.Cashier, "/api/v1/settlements/"+c.Str("id")))
	check("POST", "/api/v1/settlements/{settlement_id}/refund", f.refund(e.Manager, "", c.Str("id")))
	check("POST", "/api/v1/settlements/{settlement_id}/refund", f.refund(e.Manager, "", c.Str("id"))) // ALREADY_REFUNDED
}

// TestLoyaltyContractConformance: loyalty policy, claims, detach, member
// settlement/receipt and member history responses match OpenAPI.
func TestLoyaltyContractConformance(t *testing.T) {
	check := contract(t)
	f := setup(t)
	e := f.e
	pol := "/api/v1/branches/" + e.Branch + "/loyalty-policy"
	check("GET", "/api/v1/branches/{branch_id}/loyalty-policy", e.StaffGet(e.Cashier, pol))
	check("PUT", "/api/v1/branches/{branch_id}/loyalty-policy", e.StaffSend("PUT", e.Manager, pol, map[string]any{"expected_version": 0,
		"satang_per_point": 10000, "silver_threshold_satang": 500000, "silver_discount_bp": 300, "gold_threshold_satang": 1500000, "gold_discount_bp": 500}))
	_, ann := f.member("ann@example.com")
	_, bob := f.member("bob@example.com")
	id, v := f.order(f.tea, 2)
	check("POST", "/api/v1/visits/{visit_id}/member-claim", f.claim(ann))
	check("POST", "/api/v1/visits/{visit_id}/member-claim", f.claim(bob)) // ALREADY_CLAIMED
	check("GET", "/api/v1/visits/{visit_id}/member-claim", bob.Get(f.path("/member-claim")))
	check("GET", "/api/v1/visits/{visit_id}/bill", f.bill(e.Cashier))
	bv := f.bill(e.Cashier).Num("bill_version")
	check("POST", "/api/v1/visits/{visit_id}/member-detach", e.StaffSend("POST", e.Cashier, f.path("/member-detach"), map[string]any{"expected_version": bv, "reason": "test"}))
	f.claim(ann)
	f.serve(id, v)
	c := f.settle()
	check("POST", "/api/v1/visits/{visit_id}/settlement/confirm", c)
	check("GET", "/api/v1/settlements/{settlement_id}", e.StaffGet(e.Cashier, "/api/v1/settlements/"+c.Str("id")))
	check("GET", "/api/v1/members/me/loyalty", ann.Get("/api/v1/members/me/loyalty"))
	check("GET", "/api/v1/members/me/loyalty/entries", ann.Get("/api/v1/members/me/loyalty/entries"))
}
