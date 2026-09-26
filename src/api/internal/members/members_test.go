package members_test

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
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/epic-rinn/tableflow/src/api/internal/access"
	"github.com/epic-rinn/tableflow/src/api/internal/identity"
	"github.com/epic-rinn/tableflow/src/api/internal/members"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/app"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/dbtest"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/health"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/mail"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/password"
)

const (
	pwaOrigin = "http://pwa.test"
	pw        = "a long member password"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

type env struct {
	t    *testing.T
	pool *pgxpool.Pool
	srv  *httptest.Server
	mail *mail.Recorder
}

func newEnv(t *testing.T) *env {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), dbtest.NewMigratedDatabase(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	rec := &mail.Recorder{}
	trusted := []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}
	hasher := password.New(4)
	routes := app.Routes(health.New(pool, time.Second, quiet),
		identity.NewHTTP(identity.NewService(pool, hasher, time.Hour, 12*time.Hour), []string{"http://admin.test"}, trusted, quiet),
		access.NewHTTP(access.NewService(pool), []string{pwaOrigin}, trusted, quiet),
		members.NewHTTP(members.NewService(pool, hasher, rec, "https://pwa.example"), []string{pwaOrigin}, trusted, quiet))
	srv := httptest.NewServer(app.NewHandler(quiet, routes))
	t.Cleanup(srv.Close)
	return &env{t: t, pool: pool, srv: srv, mail: rec}
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

func (r resp) cookie() string {
	for _, c := range (&http.Response{Header: r.header}).Cookies() {
		if c.Name == members.MemberCookie && c.Value != "" {
			return c.Value
		}
	}
	return ""
}

func (e *env) req(method, path string, body any, cookie string, headers ...string) resp {
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
	if cookie != "" {
		r.Header.Set("Cookie", cookie)
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

var linkToken = regexp.MustCompile(`#([A-Za-z0-9_-]{43})`)

// lastToken extracts the token from the latest email of kind sent to addr.
func (e *env) lastToken(addr, kind string) string {
	e.t.Helper()
	msgs := e.mail.Messages(addr)
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Kind == kind {
			m := linkToken.FindStringSubmatch(msgs[i].Text)
			if m == nil {
				e.t.Fatalf("no token link in %q", msgs[i].Text)
			}
			return m[1]
		}
	}
	e.t.Fatalf("no %s email to %s (got %d messages)", kind, addr, len(msgs))
	return ""
}

func (e *env) signup(email string) {
	e.t.Helper()
	if r := e.req("POST", "/api/v1/members", map[string]any{"email": email, "password": pw, "locale": "en"}, ""); r.status != 202 {
		e.t.Fatalf("signup %s: %d %s", email, r.status, r.raw)
	}
}

func (e *env) login(email, password string) resp {
	return e.req("POST", "/api/v1/sessions/member", map[string]any{"email": email, "password": password}, "")
}

func mc(v string) string { return members.MemberCookie + "=" + v }

// TestMemberSignupVerifyLogin (ACC-002): the complete happy path.
func TestMemberSignupVerifyLogin(t *testing.T) {
	e := newEnv(t)
	e.signup("Ann@Example.com")
	tok := e.lastToken("ann@example.com", "member.verify")
	if !strings.Contains(e.mail.Messages("ann@example.com")[0].Text, "https://pwa.example/account/verify#") {
		t.Fatal("verification link does not use PWA_PUBLIC_URL fragment form")
	}

	l := e.login("ann@example.com", pw)
	if l.status != 201 || l.body["email_verified"] != false {
		t.Fatalf("unverified login: %d %s", l.status, l.raw)
	}
	set := l.header.Get("Set-Cookie")
	for _, want := range []string{members.MemberCookie + "=", "HttpOnly", "Secure", "SameSite=Lax", "Path=/"} {
		if !strings.Contains(set, want) {
			t.Errorf("cookie %q missing %q", set, want)
		}
	}
	if r := e.req("POST", "/api/v1/members/verify", map[string]any{"token": tok}, ""); r.status != 200 {
		t.Fatalf("verify: %d %s", r.status, r.raw)
	}
	me := e.req("GET", "/api/v1/members/me", nil, mc(l.cookie()))
	if me.status != 200 || me.body["email"] != "ann@example.com" || me.body["email_verified"] != true || me.body["locale"] != "en" {
		t.Fatalf("me: %d %s", me.status, me.raw)
	}
	if r := e.req("DELETE", "/api/v1/sessions/member", nil, mc(l.cookie())); r.status != 204 {
		t.Fatalf("logout: %d", r.status)
	}
	if r := e.req("GET", "/api/v1/members/me", nil, mc(l.cookie())); r.status != 401 {
		t.Fatalf("session valid after logout: %d", r.status)
	}
}

// TestMemberAccountDiscoveryIsNeutral: responses do not reveal registration.
func TestMemberAccountDiscoveryIsNeutral(t *testing.T) {
	e := newEnv(t)
	e.signup("known@example.com")
	again := e.req("POST", "/api/v1/members", map[string]any{"email": "known@example.com", "password": "another password!", "locale": "th"}, "")
	fresh := e.req("POST", "/api/v1/members", map[string]any{"email": "new@example.com", "password": pw, "locale": "en"}, "")
	if again.status != fresh.status || again.raw != fresh.raw {
		t.Fatalf("signup responses differ: %d %s / %d %s", again.status, again.raw, fresh.status, fresh.raw)
	}
	if n := len(e.mail.Messages("known@example.com")); n != 2 || e.mail.Messages("known@example.com")[1].Kind != "member.exists" {
		t.Fatalf("existing account notice not sent: %+v", e.mail.Messages("known@example.com"))
	}
	// The existing account's password was not changed by the second signup.
	if r := e.login("known@example.com", "another password!"); r.status != 401 {
		t.Fatalf("second signup changed the password: %d", r.status)
	}
	a := e.req("POST", "/api/v1/members/password-reset/request", map[string]any{"email": "known@example.com"}, "")
	b := e.req("POST", "/api/v1/members/password-reset/request", map[string]any{"email": "nobody@example.com"}, "")
	if a.status != 202 || a.raw != b.raw {
		t.Fatalf("reset responses differ: %s / %s", a.raw, b.raw)
	}
	if len(e.mail.Messages("nobody@example.com")) != 0 {
		t.Fatal("reset email sent to unknown address")
	}
	w := e.login("known@example.com", "wrong password!!")
	u := e.login("nobody@example.com", "wrong password!!")
	if w.status != 401 || w.code() != u.code() || w.code() != "INVALID_CREDENTIALS" {
		t.Fatalf("login responses differ: %s / %s", w.raw, u.raw)
	}
}

// TestMemberTokensSingleUseAndExpiry: replayed, superseded and expired links fail.
func TestMemberTokensSingleUseAndExpiry(t *testing.T) {
	e := newEnv(t)
	e.signup("b@example.com")
	first := e.lastToken("b@example.com", "member.verify")
	e.req("POST", "/api/v1/members/verification", map[string]any{"email": "b@example.com"}, "")
	second := e.lastToken("b@example.com", "member.verify")
	if r := e.req("POST", "/api/v1/members/verify", map[string]any{"token": first}, ""); r.code() != "TOKEN_INVALID" {
		t.Fatalf("superseded token accepted: %d %s", r.status, r.raw)
	}
	if r := e.req("POST", "/api/v1/members/verify", map[string]any{"token": second}, ""); r.status != 200 {
		t.Fatalf("verify: %d %s", r.status, r.raw)
	}
	if r := e.req("POST", "/api/v1/members/verify", map[string]any{"token": second}, ""); r.code() != "TOKEN_INVALID" {
		t.Fatalf("replayed token accepted: %d", r.status)
	}
	// A verify token cannot be used as a reset token.
	e.req("POST", "/api/v1/members/password-reset/request", map[string]any{"email": "b@example.com"}, "")
	reset := e.lastToken("b@example.com", "member.reset")
	if r := e.req("POST", "/api/v1/members/verify", map[string]any{"token": reset}, ""); r.code() != "TOKEN_INVALID" {
		t.Fatalf("reset token accepted for verification: %d", r.status)
	}
	if _, err := e.pool.Exec(context.Background(), "UPDATE member_tokens SET expires_at = now() - interval '1 second' WHERE purpose = 'reset'"); err != nil {
		t.Fatal(err)
	}
	if r := e.req("POST", "/api/v1/members/password-reset/confirm", map[string]any{"token": reset, "new_password": "brand new password"}, ""); r.code() != "TOKEN_INVALID" {
		t.Fatalf("expired reset token accepted: %d %s", r.status, r.raw)
	}
	var stored int
	_ = e.pool.QueryRow(context.Background(), "SELECT count(*) FROM member_tokens WHERE position(convert_to($1, 'UTF8') IN token_hash) > 0", reset).Scan(&stored)
	if stored != 0 {
		t.Fatal("raw token stored")
	}
}

// TestPasswordResetRevokesSessions: reset revokes every session, verifies
// the email and defeats an account squatter.
func TestPasswordResetRevokesSessions(t *testing.T) {
	e := newEnv(t)
	e.signup("owner@example.com") // registered by a squatter who knows pw
	squatter := e.login("owner@example.com", pw).cookie()
	other := e.login("owner@example.com", pw).cookie()

	e.req("POST", "/api/v1/members/password-reset/request", map[string]any{"email": "owner@example.com"}, "")
	tok := e.lastToken("owner@example.com", "member.reset")
	if r := e.req("POST", "/api/v1/members/password-reset/confirm", map[string]any{"token": tok, "new_password": "short"}, ""); r.status != 422 || r.code() != "VALIDATION_FAILED" {
		t.Fatalf("weak password: %d %s", r.status, r.raw)
	}
	r := e.req("POST", "/api/v1/members/password-reset/confirm", map[string]any{"token": tok, "new_password": "the real owner's password"}, "")
	if r.status != 200 {
		t.Fatalf("confirm: %d %s", r.status, r.raw)
	}
	for _, c := range []string{squatter, other} {
		if r := e.req("GET", "/api/v1/members/me", nil, mc(c)); r.status != 401 {
			t.Fatalf("session survived reset: %d", r.status)
		}
	}
	if r := e.login("owner@example.com", pw); r.status != 401 {
		t.Fatal("old password still works")
	}
	l := e.login("owner@example.com", "the real owner's password")
	if l.status != 201 || l.body["email_verified"] != true {
		t.Fatalf("login after reset: %d %s", l.status, l.raw)
	}
}

// TestConcurrentResetConfirmOneWinner: one reset token, parallel confirms.
func TestConcurrentResetConfirmOneWinner(t *testing.T) {
	e := newEnv(t)
	e.signup("c@example.com")
	e.req("POST", "/api/v1/members/password-reset/request", map[string]any{"email": "c@example.com"}, "")
	tok := e.lastToken("c@example.com", "member.reset")
	const n = 6
	codes := make([]int, n)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			<-start
			codes[i] = e.req("POST", "/api/v1/members/password-reset/confirm",
				map[string]any{"token": tok, "new_password": fmt.Sprintf("parallel password %d", i)}, "").status
		})
	}
	close(start)
	wg.Wait()
	ok := 0
	for _, c := range codes {
		if c == 200 {
			ok++
		} else if c != 422 {
			t.Errorf("unexpected %d", c)
		}
	}
	if ok != 1 {
		t.Fatalf("%d confirmations succeeded: %v", ok, codes)
	}
}

// TestMemberRateLimits (ACC-004).
func TestMemberRateLimits(t *testing.T) {
	e := newEnv(t)
	e.signup("r@example.com")
	for range 10 {
		e.login("r@example.com", "wrong password!!")
	}
	if r := e.login("r@example.com", pw); r.status != 429 || r.header.Get("Retry-After") == "" {
		t.Fatalf("login not limited: %d", r.status)
	}
	for range 5 {
		e.req("POST", "/api/v1/members/password-reset/request", map[string]any{"email": "r@example.com"}, "")
	}
	if r := e.req("POST", "/api/v1/members/password-reset/request", map[string]any{"email": "r@example.com"}, ""); r.status != 429 {
		t.Fatalf("reset request not limited: %d", r.status)
	}
	for i := range 20 {
		e.req("POST", "/api/v1/members", map[string]any{"email": fmt.Sprintf("s%d@example.com", i), "password": pw, "locale": "en"}, "", "X-Forwarded-For", "198.51.100.20")
	}
	if r := e.req("POST", "/api/v1/members", map[string]any{"email": "late@example.com", "password": pw, "locale": "en"}, "", "X-Forwarded-For", "198.51.100.20"); r.status != 429 {
		t.Fatalf("signup not limited: %d", r.status)
	}
}

// TestMemberCookieCoexistsAndIsolated: member sign-in keeps a guest session;
// member cookies do not authenticate staff or guest routes; the admin
// origin cannot send member mutations.
func TestMemberCookieCoexistsAndIsolated(t *testing.T) {
	e := newEnv(t)
	var branch string
	if err := e.pool.QueryRow(context.Background(), "INSERT INTO branches (name) VALUES ('B') RETURNING id").Scan(&branch); err != nil {
		t.Fatal(err)
	}
	var cap string
	if err := pgx.BeginFunc(context.Background(), e.pool, func(tx pgx.Tx) error {
		var err error
		cap, err = access.IssueCapability(context.Background(), tx, branch, access.KindVisit, "0198f0c0-0000-7000-8000-000000000009", nil)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	ex := e.req("POST", "/api/v1/sessions/capability", map[string]any{"token": cap, "kind": "visit"}, "")
	var g string
	for _, c := range (&http.Response{Header: ex.header}).Cookies() {
		if c.Name == access.GuestCookie {
			g = c.Value
		}
	}
	e.signup("both@example.com")
	l := e.req("POST", "/api/v1/sessions/member", map[string]any{"email": "both@example.com", "password": pw}, access.GuestCookie+"="+g)
	if l.status != 201 || strings.Contains(l.header.Get("Set-Cookie"), access.GuestCookie) {
		t.Fatalf("member login touched the guest cookie: %d %q", l.status, l.header.Get("Set-Cookie"))
	}
	both := access.GuestCookie + "=" + g + "; " + mc(l.cookie())
	if r := e.req("GET", "/api/v1/sessions/guest", nil, both); r.status != 200 {
		t.Fatalf("guest session lost: %d", r.status)
	}
	if r := e.req("GET", "/api/v1/members/me", nil, both); r.status != 200 {
		t.Fatalf("member session: %d", r.status)
	}
	for _, c := range []string{identity.StaffCookie + "=" + l.cookie(), access.GuestCookie + "=" + l.cookie()} {
		if r := e.req("GET", "/api/v1/sessions/current", nil, c); r.status != 401 {
			t.Errorf("member token accepted as staff: %d", r.status)
		}
		if r := e.req("GET", "/api/v1/sessions/guest", nil, c); r.status != 401 {
			t.Errorf("member token accepted as guest: %d", r.status)
		}
	}
	if r := e.req("GET", "/api/v1/members/me", nil, identity.StaffCookie+"="+l.cookie()); r.status != 401 {
		t.Errorf("staff-named cookie accepted as member: %d", r.status)
	}
	if r := e.req("POST", "/api/v1/sessions/member", map[string]any{"email": "both@example.com", "password": pw}, "", "Origin", "http://admin.test"); r.status != 403 {
		t.Fatalf("admin origin sent a member login: %d", r.status)
	}
}

// TestMemberSeesOnlyOwnAccount: each session resolves only to its own member.
func TestMemberSeesOnlyOwnAccount(t *testing.T) {
	e := newEnv(t)
	e.signup("x@example.com")
	e.signup("y@example.com")
	x := e.login("x@example.com", pw).cookie()
	y := e.login("y@example.com", pw).cookie()
	if r := e.req("GET", "/api/v1/members/me", nil, mc(x)); r.body["email"] != "x@example.com" {
		t.Fatalf("x sees %v", r.body["email"])
	}
	if r := e.req("GET", "/api/v1/members/me", nil, mc(y)); r.body["email"] != "y@example.com" {
		t.Fatalf("y sees %v", r.body["email"])
	}
	e.req("DELETE", "/api/v1/sessions/member", nil, mc(x))
	if r := e.req("GET", "/api/v1/members/me", nil, mc(y)); r.status != 200 {
		t.Fatal("logging out x revoked y")
	}
}

// TestMemberResponsesPrivate (no credential caching).
func TestMemberResponsesPrivate(t *testing.T) {
	e := newEnv(t)
	e.signup("p@example.com")
	l := e.login("p@example.com", pw)
	for _, r := range []resp{
		l,
		e.req("GET", "/api/v1/members/me", nil, mc(l.cookie())),
		e.req("POST", "/api/v1/members", map[string]any{"email": "q@example.com", "password": pw, "locale": "en"}, ""),
		e.req("POST", "/api/v1/members/password-reset/request", map[string]any{"email": "p@example.com"}, ""),
		e.req("GET", "/api/v1/members/me", nil, ""),
	} {
		if cc := r.header.Get("Cache-Control"); cc != "private, no-store" {
			t.Errorf("%d response Cache-Control %q", r.status, cc)
		}
		if strings.Contains(r.raw, "password") && r.status < 400 {
			t.Errorf("response mentions password: %s", r.raw)
		}
	}
}

// TestMemberContractConformance: member responses match OpenAPI.
func TestMemberContractConformance(t *testing.T) {
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
			t.Fatalf("%s %s: %d undocumented (%s)", method, path, r.status, r.raw)
		}
		media := ref.Value.Content.Get("application/json")
		if media == nil {
			return
		}
		var body any
		_ = json.Unmarshal([]byte(r.raw), &body)
		if err := media.Schema.Value.VisitJSON(body, openapi3.EnableJSONSchema2020()); err != nil {
			t.Fatalf("%s %s %d: %v\n%s", method, path, r.status, err, r.raw)
		}
	}
	check("POST", "/api/v1/members", e.req("POST", "/api/v1/members", map[string]any{"email": "k@example.com", "password": pw, "locale": "en"}, ""))
	check("POST", "/api/v1/members", e.req("POST", "/api/v1/members", map[string]any{"email": "bad", "password": "x", "locale": "fr"}, ""))
	check("POST", "/api/v1/members/verification", e.req("POST", "/api/v1/members/verification", map[string]any{"email": "k@example.com"}, ""))
	check("POST", "/api/v1/members/verify", e.req("POST", "/api/v1/members/verify", map[string]any{"token": e.lastToken("k@example.com", "member.verify")}, ""))
	check("POST", "/api/v1/members/verify", e.req("POST", "/api/v1/members/verify", map[string]any{"token": "nope"}, ""))
	l := e.login("k@example.com", pw)
	check("POST", "/api/v1/sessions/member", l)
	check("POST", "/api/v1/sessions/member", e.login("k@example.com", "wrong password!!"))
	check("GET", "/api/v1/members/me", e.req("GET", "/api/v1/members/me", nil, mc(l.cookie())))
	check("GET", "/api/v1/members/me", e.req("GET", "/api/v1/members/me", nil, ""))
	check("POST", "/api/v1/members/password-reset/request", e.req("POST", "/api/v1/members/password-reset/request", map[string]any{"email": "k@example.com"}, ""))
	check("POST", "/api/v1/members/password-reset/confirm", e.req("POST", "/api/v1/members/password-reset/confirm",
		map[string]any{"token": e.lastToken("k@example.com", "member.reset"), "new_password": "new member password"}, ""))
	check("DELETE", "/api/v1/sessions/member", e.req("DELETE", "/api/v1/sessions/member", nil, mc(e.login("k@example.com", "new member password").cookie())))
}
