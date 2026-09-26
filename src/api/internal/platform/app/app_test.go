package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/epic-rinn/tableflow/src/api/internal/platform/config"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/dbtest"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/health"
)

var discard = slog.New(slog.NewTextHandler(io.Discard, nil))

type fakePinger struct{ err error }

func (p fakePinger) Ping(context.Context) error { return p.err }

// loadContract reads the canonical contract at test time only; the API binary
// never embeds or reads specs.
func loadContract(t *testing.T) *openapi3.T {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..", "specs", "api", "openapi.yaml")
	doc, err := openapi3.NewLoader().LoadFromFile(path)
	if err != nil {
		t.Fatalf("load OpenAPI: %v", err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("OpenAPI document invalid: %v", err)
	}
	return doc
}

func conform(t *testing.T, doc *openapi3.T, path string, resp *http.Response) {
	t.Helper()
	item := doc.Paths.Find(path)
	if item == nil || item.Get == nil {
		t.Fatalf("%s: GET not documented", path)
	}
	ref := item.Get.Responses.Status(resp.StatusCode)
	if ref == nil || ref.Value == nil {
		t.Fatalf("%s: status %d not documented", path, resp.StatusCode)
	}
	media := ref.Value.Content.Get("application/json")
	if ct := resp.Header.Get("Content-Type"); ct != "application/json; charset=utf-8" || media == nil {
		t.Fatalf("%s: content type %q not documented", path, ct)
	}
	var body any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("%s: decode body: %v", path, err)
	}
	if err := media.Schema.Value.VisitJSON(body, openapi3.EnableJSONSchema2020()); err != nil {
		t.Fatalf("%s %d: body violates schema: %v", path, resp.StatusCode, err)
	}
	for name, h := range ref.Value.Headers {
		v := resp.Header.Get(name)
		if v == "" {
			t.Fatalf("%s: documented header %s missing", path, name)
		}
		if err := h.Value.Schema.Value.VisitJSON(v, openapi3.EnableJSONSchema2020()); err != nil {
			t.Fatalf("%s: header %s=%q violates schema: %v", path, name, v, err)
		}
	}
}

func do(h http.Handler, path string) *http.Response {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec.Result()
}

// HealthOpenApiValidation: the contract is valid OpenAPI 3.1, documents
// exactly the implemented routes, and actual responses conform to it.
func TestHealthOpenApiValidation(t *testing.T) {
	doc := loadContract(t)
	documented := doc.Paths.InMatchingOrder()
	slices.Sort(documented)
	implemented := []string{"/api/v1/health/live", "/api/v1/health/ready"}
	if !slices.Equal(documented, implemented) {
		t.Fatalf("documented paths %v != implemented %v", documented, implemented)
	}

	up := NewHandler(discard, health.New(fakePinger{}, time.Second, discard))
	down := NewHandler(discard, health.New(fakePinger{err: errors.New("down")}, time.Second, discard))
	conform(t, doc, "/api/v1/health/live", do(up, "/api/v1/health/live"))
	conform(t, doc, "/api/v1/health/ready", do(up, "/api/v1/health/ready"))
	conform(t, doc, "/api/v1/health/ready", do(down, "/api/v1/health/ready"))
	conform(t, doc, "/api/v1/health/live", do(down, "/api/v1/health/live"))
}

func TestUnknownRoutesAreJSON404(t *testing.T) {
	h := NewHandler(discard, health.New(fakePinger{}, time.Second, discard))
	for _, p := range []string{"/", "/api/v1/queue-tickets", "/api/v1/health/live/extra"} {
		resp := do(h, p)
		if resp.StatusCode != http.StatusNotFound || resp.Header.Get("Content-Type") != "application/json; charset=utf-8" {
			t.Errorf("%s: status %d type %q", p, resp.StatusCode, resp.Header.Get("Content-Type"))
		}
	}
}

type hangingPool struct{ release chan struct{} }

func (p hangingPool) Close() { <-p.release }

func TestClosePoolIsBounded(t *testing.T) {
	p := hangingPool{release: make(chan struct{})}
	defer close(p.release)
	start := time.Now()
	closePool(p, discard, 100*time.Millisecond)
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("closePool blocked for %v", elapsed)
	}
}

// TestRunAgainstPostgres exercises the real process wiring: migrated
// PostgreSQL, pgxpool readiness, and graceful shutdown closing the pool.
func TestRunAgainstPostgres(t *testing.T) {
	dbURL := dbtest.NewMigratedDatabase(t)
	cfg, err := config.Load(func(k string) string {
		switch k {
		case "DATABASE_URL":
			return dbURL
		case "DB_MAX_CONNS":
			return strconv.Itoa(4)
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	base := "http://" + ln.Addr().String()
	ctx, cancel := context.WithCancel(context.Background())
	runErr := make(chan error, 1)
	go func() { runErr <- Run(ctx, cfg, discard, ln) }()

	doc := loadContract(t)
	for _, p := range []string{"/api/v1/health/live", "/api/v1/health/ready"} {
		resp, err := http.Get(base + p)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s status %d", p, resp.StatusCode)
		}
		conform(t, doc, p, resp)
		resp.Body.Close()
	}

	cancel()
	select {
	case err := <-runErr:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("Run did not return after cancellation")
	}
	if _, err := http.Get(base + "/api/v1/health/live"); err == nil {
		t.Fatal("server still serving after shutdown")
	}
}
