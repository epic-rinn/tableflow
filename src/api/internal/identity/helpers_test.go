package identity_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/epic-rinn/tableflow/src/api/internal/identity"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/app"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/dbtest"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/health"
	pwhash "github.com/epic-rinn/tableflow/src/api/internal/platform/password"
)

const (
	adminOrigin = "http://admin.test"
	password    = "correct horse battery staple"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

type env struct {
	t      *testing.T
	pool   *pgxpool.Pool
	svc    *identity.Service
	srv    *httptest.Server
	branch string
}

func newEnv(t *testing.T) *env {
	t.Helper()
	url := dbtest.NewMigratedDatabase(t)
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	svc := identity.NewService(pool, pwhash.New(4), time.Hour, 12*time.Hour)
	h := identity.NewHTTP(svc, []string{adminOrigin}, []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}, quiet)
	srv := httptest.NewServer(app.NewHandler(quiet, app.Routes(health.New(pool, time.Second, quiet), h, nil, nil)))
	t.Cleanup(srv.Close)
	e := &env{t: t, pool: pool, svc: svc, srv: srv}
	branch, _, act, err := svc.Bootstrap(context.Background(), "Main", "manager@example.com", "Manager")
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	e.branch = branch
	e.activate(act.Token)
	return e
}

type resp struct {
	status int
	header http.Header
	body   map[string]any
	raw    string
}

func (r resp) errCode() string {
	if e, ok := r.body["error"].(map[string]any); ok {
		c, _ := e["code"].(string)
		return c
	}
	return ""
}

// req sends a request as a browser on the admin origin would. cookie may be "".
func (e *env) req(method, path, cookie string, body any, headers ...string) resp {
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
		r.Header.Set("Origin", adminOrigin)
	}
	if cookie != "" {
		r.Header.Set("Cookie", identity.StaffCookie+"="+cookie)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		if headers[i+1] == "" {
			r.Header.Del(headers[i])
		} else {
			r.Header.Set(headers[i], headers[i+1])
		}
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

func sessionCookie(t *testing.T, r resp) string {
	t.Helper()
	for _, c := range (&http.Response{Header: r.header}).Cookies() {
		if c.Name == identity.StaffCookie && c.Value != "" {
			return c.Value
		}
	}
	t.Fatalf("no session cookie in %d %s", r.status, r.raw)
	return ""
}

func (e *env) activate(token string) {
	e.t.Helper()
	if r := e.req("POST", "/api/v1/staff/activate", "", map[string]any{"token": token, "password": password}); r.status != 200 {
		e.t.Fatalf("activate: %d %s", r.status, r.raw)
	}
}

func (e *env) login(email string) string {
	e.t.Helper()
	r := e.req("POST", "/api/v1/sessions/staff", "", map[string]any{"email": email, "password": password})
	if r.status != 201 {
		e.t.Fatalf("login %s: %d %s", email, r.status, r.raw)
	}
	return sessionCookie(e.t, r)
}

// invite creates and activates a staff member, returning id and version.
func (e *env) invite(managerCookie, email string, roles ...string) (string, int) {
	e.t.Helper()
	r := e.req("POST", "/api/v1/branches/"+e.branch+"/staff", managerCookie,
		map[string]any{"email": email, "display_name": strings.Split(email, "@")[0], "roles": roles})
	if r.status != 201 {
		e.t.Fatalf("invite %s: %d %s", email, r.status, r.raw)
	}
	staff := r.body["staff"].(map[string]any)
	e.activate(r.body["activation"].(map[string]any)["token"].(string))
	return staff["id"].(string), int(staff["version"].(float64)) + 1 // activation bumps version
}

func (e *env) exec(sql string, args ...any) {
	e.t.Helper()
	if _, err := e.pool.Exec(context.Background(), sql, args...); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) count(sql string, args ...any) int {
	e.t.Helper()
	var n int
	if err := e.pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		e.t.Fatal(err)
	}
	return n
}
