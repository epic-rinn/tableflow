package identity_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func contract(t *testing.T) *openapi3.T {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	doc, err := openapi3.NewLoader().LoadFromFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "specs", "api", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatal(err)
	}
	return doc
}

// conform checks status, JSON body schema and documented headers.
func conform(t *testing.T, doc *openapi3.T, method, path string, r resp) {
	t.Helper()
	op := doc.Paths.Find(path).GetOperation(method)
	if op == nil {
		t.Fatalf("%s %s not documented", method, path)
	}
	ref := op.Responses.Status(r.status)
	if ref == nil || ref.Value == nil {
		t.Fatalf("%s %s: status %d not documented (%s)", method, path, r.status, r.raw)
	}
	for name, h := range ref.Value.Headers {
		if name == "Set-Cookie" {
			continue
		}
		v := r.header.Get(name)
		if err := h.Value.Schema.Value.VisitJSON(v, openapi3.EnableJSONSchema2020()); err != nil && name != "Retry-After" {
			t.Fatalf("%s %s %d: header %s=%q: %v", method, path, r.status, name, v, err)
		}
	}
	media := ref.Value.Content.Get("application/json")
	if media == nil {
		if r.raw != "" {
			t.Fatalf("%s %s %d: undocumented body %s", method, path, r.status, r.raw)
		}
		return
	}
	var body any
	if err := json.Unmarshal([]byte(r.raw), &body); err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	if err := media.Schema.Value.VisitJSON(body, openapi3.EnableJSONSchema2020()); err != nil {
		t.Fatalf("%s %s %d: body violates schema: %v\n%s", method, path, r.status, err, r.raw)
	}
}

// TestStaffContractConformance: representative success and error responses
// of every staff operation match the OpenAPI document.
func TestStaffContractConformance(t *testing.T) {
	doc := contract(t)
	e := newEnv(t)
	staffPath := "/api/v1/branches/{branch_id}/staff"
	realStaff := "/api/v1/branches/" + e.branch + "/staff"

	login := e.req("POST", "/api/v1/sessions/staff", "", map[string]any{"email": "manager@example.com", "password": password})
	conform(t, doc, "POST", "/api/v1/sessions/staff", login)
	mgr := sessionCookie(t, login)
	conform(t, doc, "POST", "/api/v1/sessions/staff", e.req("POST", "/api/v1/sessions/staff", "", map[string]any{"email": "manager@example.com", "password": "wrong password"}))
	conform(t, doc, "GET", "/api/v1/sessions/current", e.req("GET", "/api/v1/sessions/current", mgr, nil))
	conform(t, doc, "GET", "/api/v1/sessions/current", e.req("GET", "/api/v1/sessions/current", "", nil))

	inv := e.req("POST", realStaff, mgr, map[string]any{"email": "k@example.com", "display_name": "K", "roles": []string{"kitchen"}})
	conform(t, doc, "POST", staffPath, inv)
	conform(t, doc, "POST", staffPath, e.req("POST", realStaff, mgr, map[string]any{"email": "k@example.com", "display_name": "K", "roles": []string{"kitchen"}}))
	conform(t, doc, "POST", staffPath, e.req("POST", realStaff, mgr, map[string]any{"email": "bad", "display_name": "K", "roles": []string{}}))
	id := inv.body["staff"].(map[string]any)["id"].(string)

	conform(t, doc, "GET", staffPath, e.req("GET", realStaff+"?limit=1", mgr, nil))
	conform(t, doc, "POST", "/api/v1/staff/{staff_id}/activation", e.req("POST", "/api/v1/staff/"+id+"/activation", mgr, nil))
	conform(t, doc, "PATCH", "/api/v1/staff/{staff_id}/roles", e.req("PATCH", "/api/v1/staff/"+id+"/roles", mgr, map[string]any{"expected_version": 1, "roles": []string{"kitchen", "host"}}))
	conform(t, doc, "PATCH", "/api/v1/staff/{staff_id}/roles", e.req("PATCH", "/api/v1/staff/"+id+"/roles", mgr, map[string]any{"expected_version": 1, "roles": []string{"host"}}))
	conform(t, doc, "POST", "/api/v1/staff/{staff_id}/deactivate", e.req("POST", "/api/v1/staff/"+id+"/deactivate", mgr, map[string]any{"expected_version": 2, "reason": "test"}))
	conform(t, doc, "POST", "/api/v1/staff/activate", e.req("POST", "/api/v1/staff/activate", "", map[string]any{"token": "x", "password": password}))
	conform(t, doc, "DELETE", "/api/v1/sessions/current", e.req("DELETE", "/api/v1/sessions/current", mgr, nil))
	conform(t, doc, "DELETE", "/api/v1/sessions/current", e.req("DELETE", "/api/v1/sessions/current", mgr, nil, "Origin", "https://evil.example"))
}
