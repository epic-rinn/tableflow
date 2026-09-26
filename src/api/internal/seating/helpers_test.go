package seating_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/epic-rinn/tableflow/src/api/internal/access"
	"github.com/epic-rinn/tableflow/src/api/internal/identity"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/app"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/dbtest"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/health"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/idempotency"
	pwhash "github.com/epic-rinn/tableflow/src/api/internal/platform/password"
	"github.com/epic-rinn/tableflow/src/api/internal/seating"
)

const (
	adminOrigin = "http://admin.test"
	pwaOrigin   = "http://pwa.test"
	pw          = "correct horse battery staple"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

type env struct {
	t       *testing.T
	pool    *pgxpool.Pool
	srv     *httptest.Server
	idSvc   *identity.Service
	branch  string
	manager string // cookie value
	host    string
	host2   string
}

func newEnv(t *testing.T) *env {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbtest.NewMigratedDatabase(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	trusted := []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}
	idSvc := identity.NewService(pool, pwhash.New(4), time.Hour, 12*time.Hour)
	idHTTP := identity.NewHTTP(idSvc, []string{adminOrigin}, trusted, quiet)
	accHTTP := access.NewHTTP(access.NewService(pool), []string{pwaOrigin}, trusted, quiet)
	store, _ := idempotency.NewStore(bytes.Repeat([]byte{3}, 32))
	seatHTTP := seating.NewHTTP(seating.NewService(pool, idSvc), store, idHTTP, accHTTP, []string{pwaOrigin}, trusted, quiet)
	srv := httptest.NewServer(app.NewHandler(quiet, app.Routes(health.New(pool, time.Second, quiet), idHTTP, accHTTP, seatHTTP)))
	t.Cleanup(srv.Close)
	e := &env{t: t, pool: pool, srv: srv, idSvc: idSvc}

	branch, _, act, err := idSvc.Bootstrap(ctx, "Main", "manager@example.com", "Manager")
	if err != nil {
		t.Fatal(err)
	}
	if err := seating.InsertDefaultGroups(ctx, pool, branch); err != nil {
		t.Fatal(err)
	}
	e.branch = branch
	e.activate(act.Token)
	e.manager = e.login("manager@example.com")
	e.host = e.staff("host@example.com", "host")
	e.host2 = e.staff("host2@example.com", "host")
	return e
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

func (r resp) str(path ...string) string {
	var cur any = r.body
	for _, p := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = m[p]
	}
	s, _ := cur.(string)
	return s
}

func (r resp) num(path ...string) int {
	var cur any = r.body
	for _, p := range path {
		cur = cur.(map[string]any)[p]
	}
	f, _ := cur.(float64)
	return int(f)
}

func newKey() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// do sends a request. cookie is a full "name=value" pair (may be "").
// origin "" means none; GETs never send one.
func (e *env) do(method, path, cookie, origin, key string, body any, headers ...string) resp {
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
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	if cookie != "" {
		r.Header.Set("Cookie", cookie)
	}
	if key != "" {
		r.Header.Set(idempotency.Header, key)
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

func staffCookie(v string) string { return identity.StaffCookie + "=" + v }

// staffPost is a staff mutation with a fresh idempotency key.
func (e *env) staffPost(cookie, path string, body any) resp {
	return e.do("POST", path, staffCookie(cookie), adminOrigin, newKey(), body)
}

func (e *env) staffGet(cookie, path string) resp {
	return e.do("GET", path, staffCookie(cookie), "", "", nil)
}

func (e *env) activate(token string) {
	e.t.Helper()
	if r := e.do("POST", "/api/v1/staff/activate", "", adminOrigin, "", map[string]any{"token": token, "password": pw}); r.status != 200 {
		e.t.Fatalf("activate: %d %s", r.status, r.raw)
	}
}

func (e *env) login(email string) string {
	e.t.Helper()
	r := e.do("POST", "/api/v1/sessions/staff", "", adminOrigin, "", map[string]any{"email": email, "password": pw})
	for _, c := range (&http.Response{Header: r.header}).Cookies() {
		if c.Name == identity.StaffCookie && c.Value != "" {
			return c.Value
		}
	}
	e.t.Fatalf("login %s: %d %s", email, r.status, r.raw)
	return ""
}

func (e *env) staff(email string, roles ...string) string {
	e.t.Helper()
	r := e.do("POST", "/api/v1/branches/"+e.branch+"/staff", staffCookie(e.manager), adminOrigin, "",
		map[string]any{"email": email, "display_name": strings.Split(email, "@")[0], "roles": roles})
	if r.status != 201 {
		e.t.Fatalf("invite: %d %s", r.status, r.raw)
	}
	e.activate(r.str("activation", "token"))
	return e.login(email)
}

// table creates a table and returns its ID.
func (e *env) table(label string, capacity int, needs ...string) string {
	e.t.Helper()
	if needs == nil {
		needs = []string{}
	}
	r := e.do("POST", "/api/v1/branches/"+e.branch+"/tables", staffCookie(e.manager), adminOrigin, "",
		map[string]any{"label": label, "capacity": capacity, "needs": needs})
	if r.status != 201 {
		e.t.Fatalf("create table: %d %s", r.status, r.raw)
	}
	return r.str("id")
}

func (e *env) tableVersion(id string) int {
	e.t.Helper()
	var v int
	if err := e.pool.QueryRow(context.Background(), "SELECT version FROM dining_tables WHERE id = $1", id).Scan(&v); err != nil {
		e.t.Fatal(err)
	}
	return v
}

func (e *env) ticketVersion(id string) int {
	e.t.Helper()
	var v int
	if err := e.pool.QueryRow(context.Background(), "SELECT version FROM queue_tickets WHERE id = $1", id).Scan(&v); err != nil {
		e.t.Fatal(err)
	}
	return v
}

func (e *env) visitVersion(id string) int {
	e.t.Helper()
	var v int
	if err := e.pool.QueryRow(context.Background(), "SELECT version FROM visits WHERE id = $1", id).Scan(&v); err != nil {
		e.t.Fatal(err)
	}
	return v
}

func (e *env) count(sql string, args ...any) int {
	e.t.Helper()
	var n int
	if err := e.pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		e.t.Fatal(err)
	}
	return n
}

func (e *env) exec(sql string, args ...any) {
	e.t.Helper()
	if _, err := e.pool.Exec(context.Background(), sql, args...); err != nil {
		e.t.Fatal(err)
	}
}

// guest is a phone on the PWA origin with an anonymous session.
type guest struct {
	e    *env
	anon string
	sess string // guest session cookie value
}

func (e *env) newGuest() *guest {
	e.t.Helper()
	r := e.do("POST", "/api/v1/sessions/anonymous", "", pwaOrigin, "", nil)
	for _, c := range (&http.Response{Header: r.header}).Cookies() {
		if c.Name == access.AnonymousCookie {
			return &guest{e: e, anon: c.Value}
		}
	}
	e.t.Fatalf("anonymous: %d %s", r.status, r.raw)
	return nil
}

func (g *guest) join(key string, party int, needs ...string) resp {
	if needs == nil {
		needs = []string{}
	}
	return g.e.do("POST", "/api/v1/branches/"+g.e.branch+"/queue-tickets", access.AnonymousCookie+"="+g.anon, pwaOrigin, key,
		map[string]any{"party_size": party, "needs": needs})
}

// exchange trades a QR token for this phone's guest session.
func (g *guest) exchange(token, kind string) resp {
	r := g.e.do("POST", "/api/v1/sessions/capability", "", pwaOrigin, "", map[string]any{"token": token, "kind": kind})
	for _, c := range (&http.Response{Header: r.header}).Cookies() {
		if c.Name == access.GuestCookie {
			g.sess = c.Value
		}
	}
	return r
}

func (g *guest) get(path string) resp {
	return g.e.do("GET", path, access.GuestCookie+"="+g.sess, "", "", nil)
}

func (g *guest) post(path string, body any) resp {
	return g.e.do("POST", path, access.GuestCookie+"="+g.sess, pwaOrigin, newKey(), body)
}

// joinTicket joins as a fresh guest and returns ticket ID and tracking token.
func (e *env) joinTicket(party int, needs ...string) (string, string) {
	e.t.Helper()
	r := e.newGuest().join(newKey(), party, needs...)
	if r.status != 201 {
		e.t.Fatalf("join: %d %s", r.status, r.raw)
	}
	return r.str("ticket", "id"), r.str("tracking", "token")
}

func (e *env) call(cookie, ticket, table string, reason ...string) resp {
	body := map[string]any{"table_id": table, "expected_version": e.ticketVersion(ticket)}
	if len(reason) > 0 {
		body["override_reason"] = reason[0]
	}
	return e.staffPost(cookie, "/api/v1/queue-tickets/"+ticket+"/call", body)
}

func (e *env) seatBody(table string, ticket *string, party int) map[string]any {
	body := map[string]any{"branch_id": e.branch, "table_id": table, "expected_table_version": e.tableVersion(table)}
	if ticket != nil {
		body["queue_ticket_id"] = *ticket
	} else {
		body["party_size"] = party
		body["needs"] = []string{}
	}
	return body
}

// seatWalkIn seats a walk-in party and returns visit ID and dining token.
func (e *env) seatWalkIn(table string, party int) (string, string) {
	e.t.Helper()
	r := e.staffPost(e.host, "/api/v1/visits", e.seatBody(table, nil, party))
	if r.status != 201 {
		e.t.Fatalf("seat: %d %s", r.status, r.raw)
	}
	return r.str("visit", "id"), r.str("dining", "token")
}
