package httpx

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

func serve(t *testing.T, h http.Handler, req *http.Request) (*httptest.ResponseRecorder, string) {
	t.Helper()
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	rec := httptest.NewRecorder()
	Middleware(logger, h).ServeHTTP(rec, req)
	return rec, logs.String()
}

func TestRequestIDReusesSafeIncomingValue(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set(RequestIDHeader, "proxy-abc.123_Z")
	var seen string
	rec, _ := serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = RequestID(r.Context())
	}), req)
	if seen != "proxy-abc.123_Z" || rec.Header().Get(RequestIDHeader) != seen {
		t.Fatalf("request id not reused: ctx=%q header=%q", seen, rec.Header().Get(RequestIDHeader))
	}
}

func TestRequestIDReplacesUnsafeIncomingValue(t *testing.T) {
	for _, bad := range []string{"", "has space", "line\nbreak", strings.Repeat("a", 65), `"quoted"`} {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.Header.Set(RequestIDHeader, bad)
		rec, logs := serve(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), req)
		got := rec.Header().Get(RequestIDHeader)
		if got == bad || len(got) != 32 {
			t.Errorf("unsafe id %q not replaced: got %q", bad, got)
		}
		if bad != "" && strings.Contains(logs, bad) {
			t.Errorf("unsafe id %q reached logs", bad)
		}
	}
}

func TestAccessLogOmitsQueryString(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /items/{id}", func(w http.ResponseWriter, r *http.Request) {})
	req := httptest.NewRequest(http.MethodGet, "/items/42?token=super-secret", nil)
	_, logs := serve(t, mux, req)
	if strings.Contains(logs, "super-secret") || strings.Contains(logs, "/items/42") {
		t.Fatalf("log leaked path parameters or query: %s", logs)
	}
	var entry map[string]any
	if err := json.Unmarshal([]byte(logs), &entry); err != nil {
		t.Fatalf("log is not one JSON line: %v: %s", err, logs)
	}
	if entry["route"] != "GET /items/{id}" || entry["status"] != float64(200) {
		t.Fatalf("unexpected log entry: %v", entry)
	}
}

func TestPanicReturnsSanitizedJSON500(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	rec, logs := serve(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("db password=hunter2")
	}), req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	if bytes.Contains(body, []byte("hunter2")) {
		t.Fatalf("panic value leaked to client: %s", body)
	}
	var eb ErrorBody
	if err := json.Unmarshal(body, &eb); err != nil || eb.Error.Code != "INTERNAL" || eb.Error.RequestID == "" {
		t.Fatalf("bad error body %s (%v)", body, err)
	}
	if !strings.Contains(logs, "panic serving request") {
		t.Fatalf("panic not logged: %s", logs)
	}
}

func TestMethodAndNotFoundUseErrorEnvelope(t *testing.T) {
	h := Method(http.MethodGet, func(w http.ResponseWriter, r *http.Request) {})
	rec, _ := serve(t, h, httptest.NewRequest(http.MethodPost, "/x", nil))
	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET" {
		t.Fatalf("status=%d allow=%q", rec.Code, rec.Header().Get("Allow"))
	}
	rec, _ = serve(t, h, httptest.NewRequest(http.MethodHead, "/x", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("HEAD should be allowed for GET routes, got %d", rec.Code)
	}
	rec, _ = serve(t, http.HandlerFunc(NotFound), httptest.NewRequest(http.MethodGet, "/nope", nil))
	var eb ErrorBody
	if err := json.NewDecoder(rec.Body).Decode(&eb); err != nil || rec.Code != 404 || eb.Error.Code != "NOT_FOUND" {
		t.Fatalf("bad 404: %d %+v %v", rec.Code, eb, err)
	}
	if rec.Header().Get("Cache-Control") != "no-store" || rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("missing baseline headers: %v", rec.Header())
	}
}

func TestClientIPTrustsOnlyConfiguredProxies(t *testing.T) {
	proxy := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	cases := []struct {
		remote, xff string
		trusted     []netip.Prefix
		want        string
	}{
		{"203.0.113.5:1234", "6.6.6.6", nil, "203.0.113.5"},                 // untrusted peer: header ignored
		{"127.0.0.1:1234", "6.6.6.6", nil, "127.0.0.1"},                     // default trusts nobody, even loopback
		{"10.0.0.2:1234", "6.6.6.6, 198.51.100.7", proxy, "198.51.100.7"},   // right-most untrusted hop
		{"10.0.0.2:1234", "198.51.100.7, 10.0.0.9", proxy, "198.51.100.7"},  // skip inner trusted hops
		{"10.0.0.2:1234", "", proxy, "10.0.0.2"},                            // no header: the proxy itself
		{"10.0.0.2:1234", "not-an-ip, 198.51.100.7", proxy, "198.51.100.7"}, // garbage left of real hop
		{"10.0.0.2:1234", "198.51.100.7, garbage", proxy, "10.0.0.2"},       // garbage at the edge
		{"[::ffff:203.0.113.5]:1234", "6.6.6.6", nil, "203.0.113.5"},        // IPv4-mapped normalised
	}
	for _, c := range cases {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = c.remote
		if c.xff != "" {
			r.Header.Set("X-Forwarded-For", c.xff)
		}
		if got := ClientIP(r, c.trusted).String(); got != c.want {
			t.Errorf("remote=%s xff=%q: got %s want %s", c.remote, c.xff, got, c.want)
		}
	}
}
