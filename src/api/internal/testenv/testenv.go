// Package testenv starts the full API against a disposable, migrated
// PostgreSQL database for integration tests, with a bootstrapped branch,
// manager, host and kitchen staff, and helpers for guests and requests.
// It is imported only by _test packages.
package testenv

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
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/epic-rinn/tableflow/src/api/internal/access"
	"github.com/epic-rinn/tableflow/src/api/internal/billing"
	"github.com/epic-rinn/tableflow/src/api/internal/identity"
	"github.com/epic-rinn/tableflow/src/api/internal/members"
	"github.com/epic-rinn/tableflow/src/api/internal/menu"
	"github.com/epic-rinn/tableflow/src/api/internal/ordering"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/app"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/dbtest"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/health"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/idempotency"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/mail"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/password"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/token"
	"github.com/epic-rinn/tableflow/src/api/internal/reporting"
	"github.com/epic-rinn/tableflow/src/api/internal/seating"
)

// Origins used by the test server.
const (
	AdminOrigin = "http://admin.test"
	PWAOrigin   = "http://pwa.test"
	Password    = "correct horse battery staple"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

// Env is a running API with seeded staff.
type Env struct {
	T       *testing.T
	Pool    *pgxpool.Pool
	Srv     *httptest.Server
	Branch  string
	Manager string // staff cookie values
	Host    string
	Kitchen string
	Cashier string

	statements *statementCounter
}

// statementCounter counts SQL statements sent by the API's pool.
type statementCounter struct{ n atomic.Int64 }

func (c *statementCounter) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	c.n.Add(1)
	return ctx
}

func (c *statementCounter) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

// Statements returns how many statements fn caused through the API pool.
// Tests must not issue their own queries inside fn.
func (e *Env) Statements(fn func()) int64 {
	before := e.statements.n.Load()
	fn()
	return e.statements.n.Load() - before
}

// New starts the API on a fresh database.
func New(t *testing.T) *Env {
	t.Helper()
	ctx := context.Background()
	cfg, err := pgxpool.ParseConfig(dbtest.NewMigratedDatabase(t))
	if err != nil {
		t.Fatal(err)
	}
	counter := &statementCounter{}
	cfg.ConnConfig.Tracer = counter
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	trusted := []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}
	idSvc := identity.NewService(pool, password.New(4), time.Hour, 12*time.Hour)
	idHTTP := identity.NewHTTP(idSvc, []string{AdminOrigin}, trusted, quiet)
	accHTTP := access.NewHTTP(access.NewService(pool), []string{PWAOrigin}, trusted, quiet)
	store, _ := idempotency.NewStore(bytes.Repeat([]byte{5}, 32))
	memHTTP := members.NewHTTP(members.NewService(pool, password.New(4), &mail.Recorder{}, "https://pwa.test"), []string{PWAOrigin}, trusted, quiet)
	routes := app.Routes(health.New(pool, time.Second, quiet), idHTTP, accHTTP, memHTTP,
		seating.NewHTTP(seating.NewService(pool, idSvc), store, idHTTP, accHTTP, []string{PWAOrigin}, trusted, quiet),
		menu.NewHTTP(menu.NewService(pool, idSvc), idHTTP, quiet),
		ordering.NewHTTP(ordering.NewService(pool, idSvc), pool, store, idHTTP, accHTTP, []string{PWAOrigin}, quiet),
		billing.NewHTTP(billing.NewService(pool, idSvc), pool, store, idHTTP, accHTTP, memHTTP, []string{PWAOrigin}, quiet),
		reporting.NewHTTP(reporting.NewService(pool), idHTTP, quiet))
	srv := httptest.NewServer(app.NewHandler(quiet, routes))
	t.Cleanup(srv.Close)
	e := &Env{T: t, Pool: pool, Srv: srv, statements: counter}
	branch, _, act, err := idSvc.Bootstrap(ctx, "Main", "manager@example.com", "Manager")
	if err != nil {
		t.Fatal(err)
	}
	if err := seating.InsertDefaultGroups(ctx, pool, branch); err != nil {
		t.Fatal(err)
	}
	e.Branch = branch
	e.activate(act.Token)
	e.Manager = e.login("manager@example.com")
	e.Host = e.StaffMember("host@example.com", "host")
	e.Kitchen = e.StaffMember("kitchen@example.com", "kitchen")
	e.Cashier = e.StaffMember("cashier@example.com", "cashier")
	return e
}

// Resp is a decoded HTTP response.
type Resp struct {
	Status int
	Header http.Header
	Body   map[string]any
	Raw    string
}

// Code returns the error code, if any.
func (r Resp) Code() string {
	if m, ok := r.Body["error"].(map[string]any); ok {
		return m["code"].(string)
	}
	return ""
}

// Get navigates maps by keys and returns the value.
func (r Resp) Get(path ...any) any {
	var cur any = r.Body
	for _, p := range path {
		switch k := p.(type) {
		case string:
			m, ok := cur.(map[string]any)
			if !ok {
				return nil
			}
			cur = m[k]
		case int:
			a, ok := cur.([]any)
			if !ok || k >= len(a) {
				return nil
			}
			cur = a[k]
		}
	}
	return cur
}

// Str returns a string at path.
func (r Resp) Str(path ...any) string { s, _ := r.Get(path...).(string); return s }

// Num returns a number at path as int64.
func (r Resp) Num(path ...any) int64 { f, _ := r.Get(path...).(float64); return int64(f) }

// Len returns the length of an array at path.
func (r Resp) Len(path ...any) int { a, _ := r.Get(path...).([]any); return len(a) }

// NewKey returns a random UUID for Idempotency-Key.
func NewKey() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// Do sends a request; cookie is "name=value" or "", origin "" = none.
func (e *Env) Do(method, path, cookie, origin, key string, body any, headers ...string) Resp {
	e.T.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	r, _ := http.NewRequest(method, e.Srv.URL+path, rd)
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
		e.T.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	out := Resp{Status: res.StatusCode, Header: res.Header, Raw: string(raw)}
	_ = json.Unmarshal(raw, &out.Body)
	return out
}

// StaffCookie formats a staff cookie pair.
func StaffCookie(v string) string { return identity.StaffCookie + "=" + v }

// StaffGet is an authenticated staff read.
func (e *Env) StaffGet(cookie, path string) Resp {
	return e.Do("GET", path, StaffCookie(cookie), "", "", nil)
}

// StaffSend is a staff mutation with a fresh idempotency key.
func (e *Env) StaffSend(method, cookie, path string, body any) Resp {
	return e.Do(method, path, StaffCookie(cookie), AdminOrigin, NewKey(), body)
}

func (e *Env) activate(token string) {
	e.T.Helper()
	if r := e.Do("POST", "/api/v1/staff/activate", "", AdminOrigin, "", map[string]any{"token": token, "password": Password}); r.Status != 200 {
		e.T.Fatalf("activate: %d %s", r.Status, r.Raw)
	}
}

func (e *Env) login(email string) string {
	e.T.Helper()
	r := e.Do("POST", "/api/v1/sessions/staff", "", AdminOrigin, "", map[string]any{"email": email, "password": Password})
	for _, c := range (&http.Response{Header: r.Header}).Cookies() {
		if c.Name == identity.StaffCookie && c.Value != "" {
			return c.Value
		}
	}
	e.T.Fatalf("login %s: %d %s", email, r.Status, r.Raw)
	return ""
}

// StaffMember invites, activates and signs in a staff member.
func (e *Env) StaffMember(email string, roles ...string) string {
	e.T.Helper()
	r := e.Do("POST", "/api/v1/branches/"+e.Branch+"/staff", StaffCookie(e.Manager), AdminOrigin, "",
		map[string]any{"email": email, "display_name": strings.Split(email, "@")[0], "roles": roles})
	if r.Status != 201 {
		e.T.Fatalf("invite: %d %s", r.Status, r.Raw)
	}
	e.activate(r.Str("activation", "token"))
	return e.login(email)
}

// Count runs a count(*) query.
func (e *Env) Count(sql string, args ...any) int {
	e.T.Helper()
	var n int
	if err := e.Pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		e.T.Fatal(err)
	}
	return n
}

// Exec runs a statement.
func (e *Env) Exec(sql string, args ...any) {
	e.T.Helper()
	if _, err := e.Pool.Exec(context.Background(), sql, args...); err != nil {
		e.T.Fatal(err)
	}
}

// Scalar runs a single-value query.
func Scalar[T any](e *Env, sql string, args ...any) T {
	e.T.Helper()
	var v T
	if err := e.Pool.QueryRow(context.Background(), sql, args...).Scan(&v); err != nil {
		e.T.Fatal(err)
	}
	return v
}

// Table creates a table.
func (e *Env) Table(label string, capacity int) string {
	e.T.Helper()
	r := e.Do("POST", "/api/v1/branches/"+e.Branch+"/tables", StaffCookie(e.Manager), AdminOrigin, "",
		map[string]any{"label": label, "capacity": capacity, "needs": []string{}})
	if r.Status != 201 {
		e.T.Fatalf("table: %d %s", r.Status, r.Raw)
	}
	return r.Str("id")
}

// SeatWalkIn seats a walk-in party and returns visit ID and dining token.
func (e *Env) SeatWalkIn(table string, party int) (string, string) {
	e.T.Helper()
	v := Scalar[int](e, "SELECT version FROM dining_tables WHERE id = $1", table)
	r := e.StaffSend("POST", e.Host, "/api/v1/visits", map[string]any{"branch_id": e.Branch, "table_id": table,
		"expected_table_version": v, "party_size": party, "needs": []string{}})
	if r.Status != 201 {
		e.T.Fatalf("seat: %d %s", r.Status, r.Raw)
	}
	return r.Str("visit", "id"), r.Str("dining", "token")
}

// Member creates a verified member account with a live session and returns
// its ID and member cookie value (sign-in itself is covered by members tests).
func (e *Env) Member(email string) (id, cookie string) {
	e.T.Helper()
	raw, hash := token.New()
	id = Scalar[string](e, `INSERT INTO member_accounts (email, password_hash, locale, email_verified_at)
		VALUES ($1, 'argon2id$test', 'en', now()) RETURNING id::text`, email)
	e.Exec("INSERT INTO member_sessions (member_id, token_hash, expires_at) VALUES ($1, $2, now() + interval '1 day')", id, hash)
	return id, raw
}

// MemberSession issues another live session (another phone) for a member.
func (e *Env) MemberSession(memberID string) string {
	e.T.Helper()
	raw, hash := token.New()
	e.Exec("INSERT INTO member_sessions (member_id, token_hash, expires_at) VALUES ($1, $2, now() + interval '1 day')", memberID, hash)
	return raw
}

// Guest is a diner's phone on the PWA origin with a dining session.
type Guest struct {
	e      *Env
	Sess   string
	member string
}

// Diner exchanges a dining token for a new phone's guest session.
func (e *Env) Diner(token string) *Guest {
	e.T.Helper()
	r := e.Do("POST", "/api/v1/sessions/capability", "", PWAOrigin, "", map[string]any{"token": token, "kind": "visit"})
	for _, c := range (&http.Response{Header: r.Header}).Cookies() {
		if c.Name == access.GuestCookie {
			return &Guest{e: e, Sess: c.Value}
		}
	}
	e.T.Fatalf("exchange: %d %s", r.Status, r.Raw)
	return nil
}

// Get reads as this guest.
func (g *Guest) Get(path string) Resp {
	return g.e.Do("GET", path, g.cookies(), "", "", nil)
}

// WithMember adds a member cookie to this phone (claiming needs both).
func (g *Guest) WithMember(cookie string) *Guest {
	g.member = cookie
	return g
}

func (g *Guest) cookies() string {
	c := access.GuestCookie + "=" + g.Sess
	if g.member != "" {
		c += "; " + members.MemberCookie + "=" + g.member
	}
	return c
}

// Send mutates as this guest with key ("" = fresh key).
func (g *Guest) Send(path, key string, body any) Resp {
	if key == "" {
		key = NewKey()
	}
	return g.e.Do("POST", path, g.cookies(), PWAOrigin, key, body)
}

// CheckContract validates a response against the OpenAPI operation for
// method and path template (status code documented, body schema valid).
func CheckContract(t *testing.T, method, path string, r Resp) {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	doc, err := openapi3.NewLoader().LoadFromFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "specs", "api", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
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
