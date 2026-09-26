package access_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/epic-rinn/tableflow/src/api/internal/access"
)

// TestGuestContractConformance: guest/anonymous responses match OpenAPI.
func TestGuestContractConformance(t *testing.T) {
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
			t.Fatalf("%s %s: status %d undocumented (%s)", method, path, r.status, r.raw)
		}
		var body any
		_ = json.Unmarshal([]byte(r.raw), &body)
		if err := ref.Value.Content.Get("application/json").Schema.Value.VisitJSON(body, openapi3.EnableJSONSchema2020()); err != nil {
			t.Fatalf("%s %s %d: %v\n%s", method, path, r.status, err, r.raw)
		}
	}
	tok := e.issue(access.KindVisit, resource)
	ex := e.exchange(tok, access.KindVisit)
	check("POST", "/api/v1/sessions/capability", ex)
	check("POST", "/api/v1/sessions/capability", e.exchange("bad", access.KindVisit))
	check("GET", "/api/v1/sessions/guest", e.req("GET", "/api/v1/sessions/guest", nil, guest(ex.cookie(access.GuestCookie))))
	check("GET", "/api/v1/sessions/guest", e.req("GET", "/api/v1/sessions/guest", nil, nil))
	check("POST", "/api/v1/sessions/anonymous", e.req("POST", "/api/v1/sessions/anonymous", nil, nil))
	check("POST", "/api/v1/sessions/anonymous", e.req("POST", "/api/v1/sessions/anonymous", nil, nil, "Origin", "https://evil.example"))
}
