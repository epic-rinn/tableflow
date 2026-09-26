package access_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/epic-rinn/tableflow/src/api/internal/access"
	"github.com/epic-rinn/tableflow/src/api/internal/identity"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/app"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/dbtest"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/health"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/password"
)

const (
	pwaOrigin   = "http://pwa.test"
	adminOrigin = "http://admin.test"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

type env struct {
	t      *testing.T
	pool   *pgxpool.Pool
	srv    *httptest.Server
	branch string
}

func newEnv(t *testing.T) *env {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), dbtest.NewMigratedDatabase(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	trusted := []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}
	idSvc := identity.NewService(pool, password.New(2), time.Hour, 12*time.Hour)
	routes := app.Routes(health.New(pool, time.Second, quiet),
		identity.NewHTTP(idSvc, []string{adminOrigin}, trusted, quiet),
		access.NewHTTP(access.NewService(pool), []string{pwaOrigin}, trusted, quiet), nil)
	srv := httptest.NewServer(app.NewHandler(quiet, routes))
	t.Cleanup(srv.Close)
	e := &env{t: t, pool: pool, srv: srv}
	if err := pool.QueryRow(context.Background(), "INSERT INTO branches (name) VALUES ('Main') RETURNING id").Scan(&e.branch); err != nil {
		t.Fatal(err)
	}
	return e
}

func (e *env) tx(fn func(pgx.Tx) error) {
	e.t.Helper()
	if err := pgx.BeginFunc(context.Background(), e.pool, fn); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) issue(kind, resource string) string {
	e.t.Helper()
	var raw string
	e.tx(func(tx pgx.Tx) error {
		var err error
		raw, err = access.IssueCapability(context.Background(), tx, e.branch, kind, resource, nil)
		return err
	})
	return raw
}

type resp struct {
	status int
	header http.Header
	body   map[string]any
	raw    string
}

func (r resp) code() string {
	if m, ok := r.body["error"].(map[string]any); ok {
		return m["code"].(string)
	}
	return ""
}

func (r resp) cookie(name string) string {
	for _, c := range (&http.Response{Header: r.header}).Cookies() {
		if c.Name == name && c.Value != "" {
			return c.Value
		}
	}
	return ""
}

// req sends a request as a browser on the PWA origin. cookies: "name=value".
func (e *env) req(method, path string, body any, cookies []string, headers ...string) resp {
	e.t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	r, _ := http.NewRequest(method, e.srv.URL+path, rd)
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	if method != http.MethodGet {
		r.Header.Set("Origin", pwaOrigin)
	}
	if len(cookies) > 0 {
		r.Header.Set("Cookie", strings.Join(cookies, "; "))
	}
	for i := 0; i+1 < len(headers); i += 2 {
		r.Header.Set(headers[i], headers[i+1])
	}
	res, err := http.DefaultClient.Do(r)
	if err != nil {
		e.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	out := resp{status: res.StatusCode, header: res.Header, raw: string(raw)}
	_ = json.Unmarshal(raw, &out.body)
	return out
}

func (e *env) exchange(token, kind string, headers ...string) resp {
	return e.req("POST", "/api/v1/sessions/capability", map[string]any{"token": token, "kind": kind}, nil, headers...)
}

func guest(v string) []string { return []string{access.GuestCookie + "=" + v} }

const resource = "0198f0c0-0000-7000-8000-000000000001"

// TestCapabilityExchangeMultipleDiners (ACC-001): the dining token is not
// consumed; several phones obtain independent guest sessions.
func TestCapabilityExchangeMultipleDiners(t *testing.T) {
	e := newEnv(t)
	tok := e.issue(access.KindVisit, resource)
	var sessions []string
	for i := range 3 {
		r := e.exchange(tok, access.KindVisit)
		if r.status != 201 || r.body["resource_id"] != resource || r.body["kind"] != "visit" || r.body["branch_id"] != e.branch {
			t.Fatalf("diner %d: %d %s", i, r.status, r.raw)
		}
		set := r.header.Get("Set-Cookie")
		for _, want := range []string{access.GuestCookie + "=", "HttpOnly", "Secure", "SameSite=Lax", "Path=/"} {
			if !strings.Contains(set, want) {
				t.Errorf("cookie %q missing %q", set, want)
			}
		}
		if strings.Contains(r.raw, tok) {
			t.Fatal("capability token echoed")
		}
		sessions = append(sessions, r.cookie(access.GuestCookie))
	}
	for i, s := range sessions {
		r := e.req("GET", "/api/v1/sessions/guest", nil, guest(s))
		if r.status != 200 || r.body["resource_id"] != resource || r.header.Get("Cache-Control") != "private, no-store" {
			t.Fatalf("session %d: %d %s", i, r.status, r.raw)
		}
	}
}

// TestTokensStoredHashedOnly (ACC-001): neither capability nor session
// secrets appear in the database.
func TestTokensStoredHashedOnly(t *testing.T) {
	e := newEnv(t)
	tok := e.issue(access.KindQueue, resource)
	sess := e.exchange(tok, access.KindQueue).cookie(access.GuestCookie)
	for _, secret := range []string{tok, sess} {
		var n int
		err := e.pool.QueryRow(context.Background(), `
			SELECT (SELECT count(*) FROM capabilities WHERE position(convert_to($1, 'UTF8') IN token_hash) > 0)
			     + (SELECT count(*) FROM guest_sessions WHERE position(convert_to($1, 'UTF8') IN token_hash) > 0)`, secret).Scan(&n)
		if err != nil || n != 0 {
			t.Fatalf("raw secret stored (%d, %v)", n, err)
		}
	}
}

// TestRotationRevokesDerivedSessions (ACC-A2): after rotation old links and
// derived sessions fail; the new token serves several diners.
func TestRotationRevokesDerivedSessions(t *testing.T) {
	e := newEnv(t)
	old := e.issue(access.KindVisit, resource)
	s1 := e.exchange(old, access.KindVisit).cookie(access.GuestCookie)
	s2 := e.exchange(old, access.KindVisit).cookie(access.GuestCookie)

	var fresh string
	e.tx(func(tx pgx.Tx) error {
		var err error
		var gen int
		fresh, gen, err = access.RotateCapability(context.Background(), tx, access.KindVisit, resource)
		if err == nil && gen != 2 {
			err = fmt.Errorf("generation %d", gen)
		}
		return err
	})
	for _, s := range []string{s1, s2} {
		if r := e.req("GET", "/api/v1/sessions/guest", nil, guest(s)); r.status != 401 {
			t.Fatalf("pre-rotation session still valid: %d", r.status)
		}
	}
	if r := e.exchange(old, access.KindVisit); r.status != 422 || r.code() != "TOKEN_INVALID" {
		t.Fatalf("old token accepted: %d %s", r.status, r.raw)
	}
	for range 2 {
		s := e.exchange(fresh, access.KindVisit).cookie(access.GuestCookie)
		if r := e.req("GET", "/api/v1/sessions/guest", nil, guest(s)); r.status != 200 {
			t.Fatalf("new-token session invalid: %d", r.status)
		}
	}
}

// TestRotationRacingExchangeNeverLeaksOldGeneration: exchanges of the old
// token concurrent with rotation never yield a session valid afterwards.
func TestRotationRacingExchangeNeverLeaksOldGeneration(t *testing.T) {
	e := newEnv(t)
	old := e.issue(access.KindVisit, resource)
	var wg sync.WaitGroup
	start := make(chan struct{})
	sessions := make(chan string, 20)
	for range 20 {
		wg.Go(func() {
			<-start
			if c := e.exchange(old, access.KindVisit).cookie(access.GuestCookie); c != "" {
				sessions <- c
			}
		})
	}
	wg.Go(func() {
		<-start
		e.tx(func(tx pgx.Tx) error {
			_, _, err := access.RotateCapability(context.Background(), tx, access.KindVisit, resource)
			return err
		})
	})
	close(start)
	wg.Wait()
	close(sessions)
	for s := range sessions {
		if r := e.req("GET", "/api/v1/sessions/guest", nil, guest(s)); r.status != 401 {
			t.Fatalf("old-token session valid after rotation: %d", r.status)
		}
	}
}

// TestRevokedExpiredAndWrongKindRejected (ACC-001).
func TestRevokedExpiredAndWrongKindRejected(t *testing.T) {
	e := newEnv(t)
	tok := e.issue(access.KindQueue, resource)
	s := e.exchange(tok, access.KindQueue).cookie(access.GuestCookie)
	if r := e.exchange(tok, access.KindVisit); r.status != 422 || r.code() != "TOKEN_INVALID" {
		t.Fatalf("wrong kind: %d %s", r.status, r.raw)
	}
	for _, bad := range []string{"", "short", strings.Repeat("A", 43)} {
		if r := e.exchange(bad, access.KindQueue); r.code() != "TOKEN_INVALID" {
			t.Errorf("token %q: %d %s", bad, r.status, r.raw)
		}
	}
	if r := e.req("POST", "/api/v1/sessions/capability", map[string]any{"token": tok, "kind": "staff"}, nil); r.status != 422 || r.code() != "VALIDATION_FAILED" {
		t.Fatalf("invalid kind: %d %s", r.status, r.raw)
	}
	e.tx(func(tx pgx.Tx) error {
		return access.RevokeCapability(context.Background(), tx, access.KindQueue, resource)
	})
	if r := e.req("GET", "/api/v1/sessions/guest", nil, guest(s)); r.status != 401 {
		t.Fatalf("session valid after revocation: %d", r.status)
	}
	if r := e.exchange(tok, access.KindQueue); r.code() != "TOKEN_INVALID" {
		t.Fatalf("revoked token accepted: %d", r.status)
	}

	other := "0198f0c0-0000-7000-8000-000000000002"
	past := time.Now().Add(-time.Minute)
	var expired string
	e.tx(func(tx pgx.Tx) error {
		var err error
		expired, err = access.IssueCapability(context.Background(), tx, e.branch, access.KindQueue, other, &past)
		return err
	})
	if r := e.exchange(expired, access.KindQueue); r.code() != "TOKEN_INVALID" {
		t.Fatalf("expired token accepted: %d", r.status)
	}
}

// TestGuestRoutesRequirePwaOrigin (ACC-004).
func TestGuestRoutesRequirePwaOrigin(t *testing.T) {
	e := newEnv(t)
	tok := e.issue(access.KindVisit, resource)
	for name, hdr := range map[string][]string{
		"admin origin": {"Origin", adminOrigin},
		"foreign":      {"Origin", "https://evil.example"},
		"cross-site":   {"Sec-Fetch-Site", "cross-site"},
	} {
		if r := e.exchange(tok, access.KindVisit, hdr...); r.status != 403 || r.code() != "ORIGIN_REJECTED" {
			t.Errorf("%s exchange: %d %s", name, r.status, r.raw)
		}
		if r := e.req("POST", "/api/v1/sessions/anonymous", nil, nil, hdr...); r.status != 403 {
			t.Errorf("%s anonymous: %d", name, r.status)
		}
	}
	if n := func() int {
		var n int
		_ = e.pool.QueryRow(context.Background(), "SELECT count(*) FROM guest_sessions").Scan(&n)
		return n
	}(); n != 0 {
		t.Fatalf("forged requests created %d sessions", n)
	}
}

// TestCookieKindsAreIsolated (ACC-004): guest and anonymous cookies do not
// authenticate staff routes; staff cookies do not authenticate guest routes.
func TestCookieKindsAreIsolated(t *testing.T) {
	e := newEnv(t)
	tok := e.issue(access.KindVisit, resource)
	g := e.exchange(tok, access.KindVisit).cookie(access.GuestCookie)
	a := e.req("POST", "/api/v1/sessions/anonymous", nil, nil).cookie(access.AnonymousCookie)
	for _, c := range []string{access.GuestCookie + "=" + g, access.AnonymousCookie + "=" + a, identity.StaffCookie + "=" + g} {
		if r := e.req("GET", "/api/v1/sessions/current", nil, []string{c}); r.status != 401 {
			t.Errorf("%s authenticated a staff route: %d", strings.SplitN(c, "=", 2)[0], r.status)
		}
		if r := e.req("GET", "/api/v1/branches/"+e.branch+"/staff", nil, []string{c}); r.status != 401 {
			t.Errorf("%s reached staff list: %d", strings.SplitN(c, "=", 2)[0], r.status)
		}
	}
	for _, c := range []string{identity.StaffCookie + "=" + g, access.AnonymousCookie + "=" + a} {
		if r := e.req("GET", "/api/v1/sessions/guest", nil, []string{c}); r.status != 401 {
			t.Errorf("%s authenticated as guest: %d", strings.SplitN(c, "=", 2)[0], r.status)
		}
	}
}

// TestAnonymousSessionReuse: an existing valid anonymous cookie is reused.
func TestAnonymousSessionReuse(t *testing.T) {
	e := newEnv(t)
	first := e.req("POST", "/api/v1/sessions/anonymous", nil, nil)
	if first.status != 201 || first.header.Get("Cache-Control") != "private, no-store" {
		t.Fatalf("create: %d %s", first.status, first.raw)
	}
	c := first.cookie(access.AnonymousCookie)
	again := e.req("POST", "/api/v1/sessions/anonymous", nil, []string{access.AnonymousCookie + "=" + c})
	if again.status != 200 || again.cookie(access.AnonymousCookie) != "" {
		t.Fatalf("reuse: %d set-cookie=%q", again.status, again.header.Get("Set-Cookie"))
	}
	e.exec("UPDATE anonymous_sessions SET expires_at = now() - interval '1 second'")
	if r := e.req("POST", "/api/v1/sessions/anonymous", nil, []string{access.AnonymousCookie + "=" + c}); r.status != 201 {
		t.Fatalf("expired session not replaced: %d", r.status)
	}
}

func (e *env) exec(sql string) {
	e.t.Helper()
	if _, err := e.pool.Exec(context.Background(), sql); err != nil {
		e.t.Fatal(err)
	}
}

// TestCapabilityExchangeRateLimited (ACC-004).
func TestCapabilityExchangeRateLimited(t *testing.T) {
	e := newEnv(t)
	for range 120 {
		e.exchange("guess", access.KindVisit, "X-Forwarded-For", "203.0.113.50")
	}
	r := e.exchange("guess", access.KindVisit, "X-Forwarded-For", "203.0.113.50")
	if r.status != 429 || r.header.Get("Retry-After") == "" {
		t.Fatalf("not limited: %d %s", r.status, r.raw)
	}
	if r := e.exchange("guess", access.KindVisit, "X-Forwarded-For", "203.0.113.51"); r.status != 422 {
		t.Fatalf("other address limited: %d", r.status)
	}
}

// TestGuestRevalidateBlocksOnRotation: a guest mutation holding its session
// FOR SHARE makes rotation wait; afterwards the guest is rejected.
func TestGuestRevalidateBlocksOnRotation(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	tok := e.issue(access.KindVisit, resource)
	var g access.Guest
	svc := access.NewService(e.pool)
	g, _, err := svc.Exchange(ctx, tok, access.KindVisit, netip.MustParseAddr("192.0.2.9"))
	if err != nil {
		t.Fatal(err)
	}
	tx, err := e.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := access.RevalidateGuest(ctx, tx, g); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- pgx.BeginFunc(ctx, e.pool, func(rtx pgx.Tx) error {
			_, _, err := access.RotateCapability(ctx, rtx, access.KindVisit, resource)
			return err
		})
	}()
	select {
	case err := <-done:
		t.Fatalf("rotation did not wait: %v", err)
	case <-time.After(300 * time.Millisecond):
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	err = pgx.BeginFunc(ctx, e.pool, func(tx pgx.Tx) error {
		_, err := access.RevalidateGuest(ctx, tx, g)
		return err
	})
	if err != access.ErrUnauthenticated {
		t.Fatalf("guest valid after rotation: %v", err)
	}
}

// TestStaleAnonymousCookieKeepsGuestSession: rejecting an expired anonymous
// cookie clears only that cookie, never the diner's guest session.
func TestStaleAnonymousCookieKeepsGuestSession(t *testing.T) {
	e := newEnv(t)
	h := access.NewHTTP(access.NewService(e.pool), []string{pwaOrigin}, nil, quiet)
	probe := httptest.NewServer(h.RequireAnonymous(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	defer probe.Close()
	r, _ := http.NewRequest("GET", probe.URL, nil)
	r.Header.Set("Cookie", access.AnonymousCookie+"=stale; "+access.GuestCookie+"=keep")
	res, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 401 {
		t.Fatalf("status %d", res.StatusCode)
	}
	for _, c := range res.Cookies() {
		if c.Name == access.GuestCookie {
			t.Fatalf("guest cookie cleared: %v", c)
		}
	}
	if !strings.Contains(res.Header.Get("Set-Cookie"), access.AnonymousCookie+"=") {
		t.Fatalf("stale anonymous cookie not cleared: %q", res.Header.Get("Set-Cookie"))
	}
}
