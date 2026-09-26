package identity_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/epic-rinn/tableflow/src/api/internal/identity"
)

// TestStaffRoleMatrix (ACC-003, ADM-001): only managers administer staff;
// every role can read its own session.
func TestStaffRoleMatrix(t *testing.T) {
	e := newEnv(t)
	mgr := e.login("manager@example.com")
	targetID, targetVersion := e.invite(mgr, "target@example.com", "host")
	cookies := map[string]string{"manager": mgr}
	for _, role := range []string{"host", "kitchen", "cashier"} {
		email := role + "@example.com"
		e.invite(mgr, email, role)
		cookies[role] = e.login(email)
	}
	cookies["none"] = ""

	for actor, cookie := range cookies {
		manager := actor == "manager"
		checks := []struct {
			name   string
			method string
			path   string
			body   any
			want   int
		}{
			{"current", "GET", "/api/v1/sessions/current", nil, pick(cookie == "", 401, 200)},
			{"list", "GET", "/api/v1/branches/" + e.branch + "/staff", nil, pick(cookie == "", 401, pick(manager, 200, 403))},
			{"invite", "POST", "/api/v1/branches/" + e.branch + "/staff",
				map[string]any{"email": "new-" + actor + "@example.com", "display_name": "N", "roles": []string{"kitchen"}},
				pick(cookie == "", 401, pick(manager, 201, 403))},
			{"roles", "PATCH", "/api/v1/staff/" + targetID + "/roles",
				map[string]any{"expected_version": targetVersion, "roles": []string{"host", "kitchen"}},
				pick(cookie == "", 401, pick(manager, 200, 403))},
		}
		for _, c := range checks {
			r := e.req(c.method, c.path, cookie, c.body)
			if r.status != c.want {
				t.Errorf("%s %s: got %d want %d (%s)", actor, c.name, r.status, c.want, r.raw)
			}
			if c.name == "roles" && r.status == 200 {
				targetVersion = int(r.body["version"].(float64))
			}
		}
	}
}

func pick(cond bool, a, b int) int {
	if cond {
		return a
	}
	return b
}

// TestCrossBranchStaffAccessIsHidden (ACC-A1): a manager of another branch
// gets 404 for this branch's staff, reads and writes alike.
func TestCrossBranchStaffAccessIsHidden(t *testing.T) {
	e := newEnv(t)
	mgr := e.login("manager@example.com")
	victimID, v := e.invite(mgr, "victim@example.com", "host")

	// Second branch with its own manager, created directly (bootstrap is one-shot).
	var other, otherMgr string
	ctx := context.Background()
	if err := e.pool.QueryRow(ctx, "INSERT INTO branches (name) VALUES ('Other') RETURNING id").Scan(&other); err != nil {
		t.Fatal(err)
	}
	if err := e.pool.QueryRow(ctx, `INSERT INTO staff_accounts (branch_id, email, display_name, status, password_hash)
		SELECT $1, 'other@example.com', 'Other', 'active', password_hash FROM staff_accounts WHERE email = 'manager@example.com'
		RETURNING id`, other).Scan(&otherMgr); err != nil {
		t.Fatal(err)
	}
	e.exec("INSERT INTO staff_roles VALUES ($1, 'manager')", otherMgr)
	intruder := e.login("other@example.com")

	for _, c := range []struct {
		method, path string
		body         any
	}{
		{"GET", "/api/v1/branches/" + e.branch + "/staff", nil},
		{"POST", "/api/v1/branches/" + e.branch + "/staff", map[string]any{"email": "x@example.com", "display_name": "X", "roles": []string{"host"}}},
		{"PATCH", "/api/v1/staff/" + victimID + "/roles", map[string]any{"expected_version": v, "roles": []string{"manager"}}},
		{"POST", "/api/v1/staff/" + victimID + "/deactivate", map[string]any{"expected_version": v, "reason": "x"}},
		{"POST", "/api/v1/staff/" + victimID + "/activation", nil},
	} {
		r := e.req(c.method, c.path, intruder, c.body)
		if r.status != 404 || r.errCode() != "NOT_FOUND" {
			t.Errorf("%s %s: %d %s", c.method, c.path, r.status, r.raw)
		}
	}
	if n := e.count("SELECT count(*) FROM staff_roles WHERE staff_account_id = $1", victimID); n != 1 {
		t.Fatalf("victim roles changed: %d", n)
	}
	// Unknown and malformed IDs look the same as other-branch IDs.
	for _, p := range []string{"/api/v1/branches/not-a-uuid/staff", "/api/v1/branches/00000000-0000-0000-0000-000000000000/staff"} {
		if r := e.req("GET", p, mgr, nil); r.status != 404 {
			t.Errorf("%s: %d", p, r.status)
		}
	}
}

// TestMutationsRequireAllowedOrigin (ACC-A3, ACC-004).
func TestMutationsRequireAllowedOrigin(t *testing.T) {
	e := newEnv(t)
	mgr := e.login("manager@example.com")
	invite := map[string]any{"email": "o@example.com", "display_name": "O", "roles": []string{"host"}}
	for name, hdr := range map[string][]string{
		"missing origin":   {"Origin", ""},
		"forged origin":    {"Origin", "https://evil.example"},
		"pwa origin":       {"Origin", "http://localhost:3000"},
		"cross-site fetch": {"Sec-Fetch-Site", "cross-site"},
		"same-site fetch":  {"Sec-Fetch-Site", "same-site"},
	} {
		if r := e.req("POST", "/api/v1/branches/"+e.branch+"/staff", mgr, invite, hdr...); r.status != 403 || r.errCode() != "ORIGIN_REJECTED" {
			t.Errorf("%s invite: %d %s", name, r.status, r.raw)
		}
		if r := e.req("POST", "/api/v1/sessions/staff", "", map[string]any{"email": "manager@example.com", "password": password}, hdr...); r.status != 403 {
			t.Errorf("%s login: %d", name, r.status)
		}
		if r := e.req("DELETE", "/api/v1/sessions/current", mgr, nil, hdr...); r.status != 403 {
			t.Errorf("%s logout: %d", name, r.status)
		}
	}
	if n := e.count("SELECT count(*) FROM staff_accounts WHERE email = 'o@example.com'"); n != 0 {
		t.Fatal("forged request created an account")
	}
	if r := e.req("POST", "/api/v1/branches/"+e.branch+"/staff", mgr, invite, "Sec-Fetch-Site", "same-origin"); r.status != 201 {
		t.Fatalf("same-origin request rejected: %d %s", r.status, r.raw)
	}
}

// TestNoPublicStaffRegistration (ACC-002): no unauthenticated path creates staff.
func TestNoPublicStaffRegistration(t *testing.T) {
	e := newEnv(t)
	body := map[string]any{"email": "self@example.com", "display_name": "Self", "roles": []string{"manager"}}
	for _, p := range []string{"/api/v1/branches/" + e.branch + "/staff", "/api/v1/staff", "/api/v1/staff/register"} {
		if r := e.req("POST", p, "", body); r.status != 401 && r.status != 404 {
			t.Errorf("%s: %d", p, r.status)
		}
	}
	if n := e.count("SELECT count(*) FROM staff_accounts WHERE email = 'self@example.com'"); n != 0 {
		t.Fatal("self-registration created an account")
	}
	// Bootstrap is one-shot.
	if _, _, _, err := e.svc.Bootstrap(context.Background(), "Second", "second@example.com", "S"); err != identity.ErrBootstrapDone {
		t.Fatalf("second bootstrap: %v", err)
	}
}

// TestActivationTokenSingleUse (ACC-002): used, reissued-over and expired
// tokens are rejected identically; the account can then log in.
func TestActivationTokenSingleUse(t *testing.T) {
	e := newEnv(t)
	mgr := e.login("manager@example.com")
	r := e.req("POST", "/api/v1/branches/"+e.branch+"/staff", mgr, map[string]any{"email": "a@example.com", "display_name": "A", "roles": []string{"host"}})
	id := r.body["staff"].(map[string]any)["id"].(string)
	first := r.body["activation"].(map[string]any)["token"].(string)

	// Weak password is a field error and does not consume the token.
	if r := e.req("POST", "/api/v1/staff/activate", "", map[string]any{"token": first, "password": "short"}); r.status != 422 || r.errCode() != "VALIDATION_FAILED" {
		t.Fatalf("weak password: %d %s", r.status, r.raw)
	}
	// Reissue invalidates the first token.
	re := e.req("POST", "/api/v1/staff/"+id+"/activation", mgr, nil)
	if re.status != 201 {
		t.Fatalf("reissue: %d %s", re.status, re.raw)
	}
	second := re.body["token"].(string)
	if r := e.req("POST", "/api/v1/staff/activate", "", map[string]any{"token": first, "password": password}); r.errCode() != "TOKEN_INVALID" {
		t.Fatalf("revoked token accepted: %d %s", r.status, r.raw)
	}
	e.activate(second)
	if r := e.req("POST", "/api/v1/staff/activate", "", map[string]any{"token": second, "password": password}); r.errCode() != "TOKEN_INVALID" {
		t.Fatalf("replayed token accepted: %d %s", r.status, r.raw)
	}
	e.login("a@example.com")
	if r := e.req("POST", "/api/v1/staff/"+id+"/activation", mgr, nil); r.status != 409 || r.errCode() != "NOT_INVITED" {
		t.Fatalf("reissue for active account: %d %s", r.status, r.raw)
	}

	// Expired token.
	r = e.req("POST", "/api/v1/branches/"+e.branch+"/staff", mgr, map[string]any{"email": "b@example.com", "display_name": "B", "roles": []string{"host"}})
	expired := r.body["activation"].(map[string]any)["token"].(string)
	e.exec("UPDATE staff_activation_tokens SET expires_at = now() - interval '1 second' WHERE staff_account_id = $1", r.body["staff"].(map[string]any)["id"])
	for _, tok := range []string{expired, "garbage", strings.Repeat("A", 43)} {
		if r := e.req("POST", "/api/v1/staff/activate", "", map[string]any{"token": tok, "password": password}); r.status != 422 || r.errCode() != "TOKEN_INVALID" {
			t.Errorf("token %q: %d %s", tok, r.status, r.raw)
		}
	}
}

// TestConcurrentActivationOneWinner: simultaneous activations of one token
// on independent connections produce exactly one success.
func TestConcurrentActivationOneWinner(t *testing.T) {
	e := newEnv(t)
	mgr := e.login("manager@example.com")
	r := e.req("POST", "/api/v1/branches/"+e.branch+"/staff", mgr, map[string]any{"email": "c@example.com", "display_name": "C", "roles": []string{"host"}})
	token := r.body["activation"].(map[string]any)["token"].(string)

	const n = 8
	var wg sync.WaitGroup
	start := make(chan struct{})
	codes := make([]int, n)
	for i := range n {
		wg.Go(func() {
			<-start
			codes[i] = e.req("POST", "/api/v1/staff/activate", "", map[string]any{"token": token, "password": fmt.Sprintf("%s-%d", password, i)}).status
		})
	}
	close(start)
	wg.Wait()
	ok := 0
	for _, c := range codes {
		switch c {
		case 200:
			ok++
		case 422:
		default:
			t.Errorf("unexpected status %d", c)
		}
	}
	if ok != 1 {
		t.Fatalf("%d activations succeeded: %v", ok, codes)
	}
	if n := e.count("SELECT count(*) FROM staff_activation_tokens WHERE used_at IS NOT NULL AND staff_account_id = (SELECT id FROM staff_accounts WHERE email = 'c@example.com')"); n != 1 {
		t.Fatalf("used tokens = %d", n)
	}
}

// TestSessionCookieAttributes and TestAuthenticatedResponsesArePrivate (ACC-004).
func TestSessionCookieAttributes(t *testing.T) {
	e := newEnv(t)
	r := e.req("POST", "/api/v1/sessions/staff", "", map[string]any{"email": "  Manager@Example.com ", "password": password})
	if r.status != 201 {
		t.Fatalf("login: %d %s", r.status, r.raw)
	}
	set := r.header.Get("Set-Cookie")
	for _, want := range []string{identity.StaffCookie + "=", "Path=/", "HttpOnly", "Secure", "SameSite=Strict", "Expires="} {
		if !strings.Contains(set, want) {
			t.Errorf("Set-Cookie %q missing %q", set, want)
		}
	}
	if strings.Contains(strings.ToLower(set), "domain=") {
		t.Errorf("cookie must be host-only: %q", set)
	}
	raw := sessionCookie(t, r)
	if n := e.count("SELECT count(*) FROM staff_sessions WHERE token_hash = convert_to($1, 'UTF8')", raw); n != 0 {
		t.Fatal("raw session token stored")
	}
	if strings.Contains(r.raw, raw) {
		t.Fatal("session token echoed in body")
	}
}

func TestAuthenticatedResponsesArePrivate(t *testing.T) {
	e := newEnv(t)
	mgr := e.login("manager@example.com")
	for _, r := range []resp{
		e.req("GET", "/api/v1/sessions/current", mgr, nil),
		e.req("GET", "/api/v1/branches/"+e.branch+"/staff", mgr, nil),
		e.req("GET", "/api/v1/sessions/current", "", nil),
		e.req("POST", "/api/v1/sessions/staff", "", map[string]any{"email": "manager@example.com", "password": "wrong password!"}),
	} {
		if cc := r.header.Get("Cache-Control"); cc != "private, no-store" {
			t.Errorf("%d response Cache-Control %q", r.status, cc)
		}
	}
}

// TestLogoutRevokes and TestSessionExpiry (ACC-004).
func TestLogoutRevokes(t *testing.T) {
	e := newEnv(t)
	mgr := e.login("manager@example.com")
	other := e.login("manager@example.com")
	r := e.req("DELETE", "/api/v1/sessions/current", mgr, nil)
	if r.status != 204 || !strings.Contains(r.header.Get("Set-Cookie"), "Max-Age=0") {
		t.Fatalf("logout: %d %q", r.status, r.header.Get("Set-Cookie"))
	}
	if r := e.req("GET", "/api/v1/sessions/current", mgr, nil); r.status != 401 {
		t.Fatalf("revoked cookie accepted: %d", r.status)
	}
	if r := e.req("GET", "/api/v1/sessions/current", other, nil); r.status != 200 {
		t.Fatalf("logout revoked a different session: %d", r.status)
	}
}

func TestSessionExpiry(t *testing.T) {
	e := newEnv(t)
	idle := e.login("manager@example.com")
	absolute := e.login("manager@example.com")
	fresh := e.login("manager@example.com")
	e.exec("UPDATE staff_sessions SET last_seen_at = now() - interval '61 minutes' WHERE id = (SELECT id FROM staff_sessions ORDER BY created_at LIMIT 1)")
	e.exec("UPDATE staff_sessions SET expires_at = now() - interval '1 second' WHERE id = (SELECT id FROM staff_sessions ORDER BY created_at LIMIT 1 OFFSET 1)")
	for name, c := range map[string]string{"idle": idle, "absolute": absolute} {
		if r := e.req("GET", "/api/v1/sessions/current", c, nil); r.status != 401 {
			t.Errorf("%s-expired session: %d", name, r.status)
		}
	}
	// A stale-but-valid session is touched at most once per minute.
	e.exec("UPDATE staff_sessions SET last_seen_at = now() - interval '5 minutes' WHERE id = (SELECT id FROM staff_sessions ORDER BY created_at LIMIT 1 OFFSET 2)")
	if r := e.req("GET", "/api/v1/sessions/current", fresh, nil); r.status != 200 {
		t.Fatalf("valid session rejected: %d", r.status)
	}
	if n := e.count("SELECT count(*) FROM staff_sessions WHERE last_seen_at > now() - interval '1 minute' AND id = (SELECT id FROM staff_sessions ORDER BY created_at LIMIT 1 OFFSET 2)"); n != 1 {
		t.Fatal("stale session was not touched")
	}
}

// TestLoginRateLimited and TestActivationRateLimited (ACC-004).
func TestLoginRateLimited(t *testing.T) {
	e := newEnv(t)
	wrong := map[string]any{"email": "manager@example.com", "password": "not the password"}
	for i := range 10 {
		if r := e.req("POST", "/api/v1/sessions/staff", "", wrong); r.status != 401 || r.errCode() != "INVALID_CREDENTIALS" {
			t.Fatalf("attempt %d: %d %s", i+1, r.status, r.raw)
		}
	}
	r := e.req("POST", "/api/v1/sessions/staff", "", map[string]any{"email": "manager@example.com", "password": password})
	if r.status != 429 || r.errCode() != "RATE_LIMITED" || r.header.Get("Retry-After") == "" {
		t.Fatalf("11th attempt (correct password) not limited: %d %s", r.status, r.raw)
	}
	// Unknown accounts respond identically to wrong passwords.
	if r := e.req("POST", "/api/v1/sessions/staff", "", map[string]any{"email": "nobody@example.com", "password": "whatever pass"}); r.status != 401 || r.errCode() != "INVALID_CREDENTIALS" {
		t.Fatalf("unknown account: %d %s", r.status, r.raw)
	}
	// Per-IP limit applies across emails; X-Forwarded-For from the trusted
	// proxy (loopback in these tests) selects the client address.
	for i := range 100 {
		e.req("POST", "/api/v1/sessions/staff", "", map[string]any{"email": fmt.Sprintf("u%d@example.com", i), "password": "whatever pass"}, "X-Forwarded-For", "203.0.113.9")
	}
	if r := e.req("POST", "/api/v1/sessions/staff", "", map[string]any{"email": "fresh@example.com", "password": "x"}, "X-Forwarded-For", "203.0.113.9"); r.status != 429 {
		t.Fatalf("per-IP limit not applied: %d", r.status)
	}
	if r := e.req("POST", "/api/v1/sessions/staff", "", map[string]any{"email": "fresh@example.com", "password": "whatever pass"}, "X-Forwarded-For", "203.0.113.10"); r.status != 401 {
		t.Fatalf("other client IP limited: %d", r.status)
	}
	if n := e.count("SELECT count(*) FROM auth_throttle WHERE bucket LIKE '%manager@example.com%'"); n != 0 {
		t.Fatal("raw email stored in throttle bucket")
	}
}

func TestActivationRateLimited(t *testing.T) {
	e := newEnv(t)
	for range 20 {
		e.req("POST", "/api/v1/staff/activate", "", map[string]any{"token": "bad", "password": password}, "X-Forwarded-For", "198.51.100.7")
	}
	if r := e.req("POST", "/api/v1/staff/activate", "", map[string]any{"token": "bad", "password": password}, "X-Forwarded-For", "198.51.100.7"); r.status != 429 {
		t.Fatalf("activation not limited: %d", r.status)
	}
}

// TestLastManagerProtected: demoting or deactivating the only active manager fails.
func TestLastManagerProtected(t *testing.T) {
	e := newEnv(t)
	mgr := e.login("manager@example.com")
	cur := e.req("GET", "/api/v1/sessions/current", mgr, nil)
	selfID := cur.body["staff_id"].(string)
	var v int
	if err := e.pool.QueryRow(context.Background(), "SELECT version FROM staff_accounts WHERE id = $1", selfID).Scan(&v); err != nil {
		t.Fatal(err)
	}
	if r := e.req("PATCH", "/api/v1/staff/"+selfID+"/roles", mgr, map[string]any{"expected_version": v, "roles": []string{"host"}}); r.status != 409 || r.errCode() != "LAST_MANAGER" {
		t.Fatalf("self-demotion: %d %s", r.status, r.raw)
	}
	if r := e.req("POST", "/api/v1/staff/"+selfID+"/deactivate", mgr, map[string]any{"expected_version": v, "reason": "leaving"}); r.status != 409 || r.errCode() != "LAST_MANAGER" {
		t.Fatalf("self-deactivation: %d %s", r.status, r.raw)
	}
	if r := e.req("PATCH", "/api/v1/staff/"+selfID+"/roles", mgr, map[string]any{"expected_version": v + 5, "roles": []string{"manager"}}); r.status != 409 || r.errCode() != "VERSION_CONFLICT" {
		t.Fatalf("stale version: %d %s", r.status, r.raw)
	}
	if r := e.req("GET", "/api/v1/sessions/current", mgr, nil); r.status != 200 {
		t.Fatal("rolled-back change revoked the session")
	}
}

// TestConcurrentMutualDemotion: two managers demoting each other at once
// cannot leave the branch without a manager; the loser observes revocation.
func TestConcurrentMutualDemotion(t *testing.T) {
	for round := range 5 {
		e := newEnv(t)
		a := e.login("manager@example.com")
		bID, bV := e.invite(a, fmt.Sprintf("b%d@example.com", round), "manager")
		b := e.login(fmt.Sprintf("b%d@example.com", round))
		aID := e.req("GET", "/api/v1/sessions/current", a, nil).body["staff_id"].(string)
		var aV int
		_ = e.pool.QueryRow(context.Background(), "SELECT version FROM staff_accounts WHERE id = $1", aID).Scan(&aV)

		start := make(chan struct{})
		var wg sync.WaitGroup
		var ra, rb resp
		wg.Go(func() {
			<-start
			ra = e.req("PATCH", "/api/v1/staff/"+bID+"/roles", a, map[string]any{"expected_version": bV, "roles": []string{"host"}})
		})
		wg.Go(func() {
			<-start
			rb = e.req("PATCH", "/api/v1/staff/"+aID+"/roles", b, map[string]any{"expected_version": aV, "roles": []string{"host"}})
		})
		close(start)
		wg.Wait()

		managers := e.count(`SELECT count(*) FROM staff_accounts s WHERE status = 'active'
			AND EXISTS (SELECT 1 FROM staff_roles r WHERE r.staff_account_id = s.id AND r.role = 'manager')`)
		if managers != 1 {
			t.Fatalf("round %d: %d managers remain (a=%d %s, b=%d %s)", round, managers, ra.status, ra.raw, rb.status, rb.raw)
		}
		if (ra.status == 200) == (rb.status == 200) {
			t.Fatalf("round %d: expected exactly one success: a=%d b=%d", round, ra.status, rb.status)
		}
		loser := rb
		if rb.status == 200 {
			loser = ra
		}
		if loser.status != 401 {
			t.Fatalf("round %d: loser status %d %s", round, loser.status, loser.raw)
		}
	}
}

// TestRoleChangeRevokesSessions (ACC-A3, ADM-A4): the target's existing
// sessions stop working immediately; new logins carry the new roles.
func TestRoleChangeRevokesSessions(t *testing.T) {
	e := newEnv(t)
	mgr := e.login("manager@example.com")
	id, v := e.invite(mgr, "k@example.com", "kitchen", "manager")
	k := e.login("k@example.com")
	if r := e.req("GET", "/api/v1/branches/"+e.branch+"/staff", k, nil); r.status != 200 {
		t.Fatalf("manager role not effective: %d", r.status)
	}
	if r := e.req("PATCH", "/api/v1/staff/"+id+"/roles", mgr, map[string]any{"expected_version": v, "roles": []string{"kitchen"}}); r.status != 200 {
		t.Fatalf("role change: %d %s", r.status, r.raw)
	}
	if r := e.req("GET", "/api/v1/branches/"+e.branch+"/staff", k, nil); r.status != 401 {
		t.Fatalf("old session still valid after role change: %d", r.status)
	}
	k2 := e.login("k@example.com")
	if r := e.req("GET", "/api/v1/branches/"+e.branch+"/staff", k2, nil); r.status != 403 {
		t.Fatalf("revoked role still authorised: %d", r.status)
	}
	cur := e.req("GET", "/api/v1/sessions/current", k2, nil)
	if roles := fmt.Sprint(cur.body["roles"]); roles != "[kitchen]" {
		t.Fatalf("roles = %s", roles)
	}

	// Deactivation revokes sessions and blocks login.
	v = int(e.req("GET", "/api/v1/branches/"+e.branch+"/staff", mgr, nil).body["items"].([]any)[1].(map[string]any)["version"].(float64))
	if r := e.req("POST", "/api/v1/staff/"+id+"/deactivate", mgr, map[string]any{"expected_version": v, "reason": "left the restaurant"}); r.status != 200 {
		t.Fatalf("deactivate: %d %s", r.status, r.raw)
	}
	if r := e.req("GET", "/api/v1/sessions/current", k2, nil); r.status != 401 {
		t.Fatalf("deactivated session valid: %d", r.status)
	}
	if r := e.req("POST", "/api/v1/sessions/staff", "", map[string]any{"email": "k@example.com", "password": password}); r.status != 401 || r.errCode() != "INVALID_CREDENTIALS" {
		t.Fatalf("deactivated login: %d %s", r.status, r.raw)
	}
	if r := e.req("PATCH", "/api/v1/staff/"+id+"/roles", mgr, map[string]any{"expected_version": v + 1, "roles": []string{"host"}}); r.status != 409 || r.errCode() != "ACCOUNT_DISABLED" {
		t.Fatalf("roles on disabled account: %d %s", r.status, r.raw)
	}
}

// TestStaffAdministrationAudited (OPS-001): events carry actor, action,
// resource, request ID and reason, and never secrets.
func TestStaffAdministrationAudited(t *testing.T) {
	e := newEnv(t)
	mgr := e.login("manager@example.com")
	id, v := e.invite(mgr, "aud@example.com", "host")
	e.req("PATCH", "/api/v1/staff/"+id+"/roles", mgr, map[string]any{"expected_version": v, "roles": []string{"cashier"}}, "X-Request-ID", "req-roles-1")
	e.req("POST", "/api/v1/staff/"+id+"/deactivate", mgr, map[string]any{"expected_version": v + 1, "reason": "seasonal contract ended"})

	rows, err := e.pool.Query(context.Background(),
		"SELECT action, coalesce(actor_staff_id::text, ''), request_id, coalesce(reason, ''), details::text FROM audit_events WHERE resource_id = $1 ORDER BY occurred_at, id", id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var action, actor, reqID, reason, details string
		if err := rows.Scan(&action, &actor, &reqID, &reason, &details); err != nil {
			t.Fatal(err)
		}
		if reqID == "" || (action != "staff.activated" && actor == "") {
			t.Errorf("%s missing actor/request id", action)
		}
		if strings.Contains(details, "token") || strings.Contains(details, "password") {
			t.Errorf("%s details leak secrets: %s", action, details)
		}
		got = append(got, action+"|"+reason)
		if action == "staff.roles_changed" && reqID != "req-roles-1" {
			t.Errorf("request id not recorded: %s", reqID)
		}
	}
	want := "[staff.invited| staff.activated| staff.roles_changed| staff.deactivated|seasonal contract ended]"
	if fmt.Sprint(got) != want {
		t.Fatalf("audit trail %v\nwant %s", got, want)
	}
}

// TestStaffListPagination: keyset pages are stable and cursors are bound to the branch.
func TestStaffListPagination(t *testing.T) {
	e := newEnv(t)
	mgr := e.login("manager@example.com")
	for i := range 5 {
		e.req("POST", "/api/v1/branches/"+e.branch+"/staff", mgr, map[string]any{"email": fmt.Sprintf("p%d@example.com", i), "display_name": "P", "roles": []string{"host"}})
	}
	seen := map[string]bool{}
	cursor := ""
	for pages := 0; ; pages++ {
		path := "/api/v1/branches/" + e.branch + "/staff?limit=2"
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		r := e.req("GET", path, mgr, nil)
		if r.status != 200 {
			t.Fatalf("page %d: %d %s", pages, r.status, r.raw)
		}
		for _, it := range r.body["items"].([]any) {
			id := it.(map[string]any)["id"].(string)
			if seen[id] {
				t.Fatalf("duplicate %s", id)
			}
			seen[id] = true
		}
		next, _ := r.body["next_cursor"].(string)
		if next == "" {
			break
		}
		cursor = next
	}
	if len(seen) != 6 {
		t.Fatalf("saw %d accounts, want 6", len(seen))
	}
	for _, bad := range []string{"garbage", "eyJiIjoiMDAwMDAwMDAtMDAwMC0wMDAwLTAwMDAtMDAwMDAwMDAwMDAwIiwidCI6IjIwMjYtMDEtMDFUMDA6MDA6MDBaIiwiaSI6IjAwMDAwMDAwLTAwMDAtMDAwMC0wMDAwLTAwMDAwMDAwMDAwMCJ9"} {
		if r := e.req("GET", "/api/v1/branches/"+e.branch+"/staff?cursor="+bad, mgr, nil); r.status != 400 || r.errCode() != "INVALID_CURSOR" {
			t.Errorf("cursor %q: %d %s", bad, r.status, r.raw)
		}
	}
	if r := e.req("GET", "/api/v1/branches/"+e.branch+"/staff?limit=101", mgr, nil); r.status != 422 {
		t.Errorf("limit 101: %d", r.status)
	}
}

// TestRequestValidation: body limits, unknown fields and content type.
func TestRequestValidation(t *testing.T) {
	e := newEnv(t)
	mgr := e.login("manager@example.com")
	path := "/api/v1/branches/" + e.branch + "/staff"
	cases := []struct {
		body any
		hdr  []string
		want int
	}{
		{map[string]any{"email": "v@example.com", "display_name": "V", "roles": []string{"host"}, "role": "manager"}, nil, 400},
		{map[string]any{"email": "v@example.com", "display_name": "V", "roles": []string{"owner"}}, nil, 422},
		{map[string]any{"email": "not-an-email", "display_name": "V", "roles": []string{"host"}}, nil, 422},
		{map[string]any{"email": "v@example.com", "display_name": strings.Repeat("x", 20000), "roles": []string{"host"}}, nil, 413},
		{map[string]any{"email": "v@example.com", "display_name": "V", "roles": []string{"host"}}, []string{"Content-Type", "text/plain"}, 415},
	}
	for i, c := range cases {
		if r := e.req("POST", path, mgr, c.body, c.hdr...); r.status != c.want {
			t.Errorf("case %d: %d want %d (%s)", i, r.status, c.want, r.raw)
		}
	}
	if r := e.req("POST", path, mgr, map[string]any{"email": "manager@example.com", "display_name": "Dup", "roles": []string{"host"}}); r.status != 409 || r.errCode() != "EMAIL_TAKEN" {
		t.Errorf("duplicate email: %d %s", r.status, r.raw)
	}
	if r := e.req(http.MethodPut, "/api/v1/sessions/current", mgr, nil); r.status != 405 {
		t.Errorf("PUT current: %d", r.status)
	}
}

// TestPurgeRemovesOnlyStaleRows: maintenance deletes old throttle buckets
// and sessions expired over 30 days, keeping everything else.
func TestPurgeRemovesOnlyStaleRows(t *testing.T) {
	e := newEnv(t)
	e.login("manager@example.com")
	e.login("manager@example.com")
	e.exec("UPDATE staff_sessions SET expires_at = now() - interval '31 days' WHERE id = (SELECT id FROM staff_sessions ORDER BY created_at LIMIT 1)")
	e.exec("INSERT INTO auth_throttle VALUES ('old', now() - interval '2 days', 3), ('new', now(), 1)")
	n, err := e.svc.Purge(context.Background())
	if err != nil || n != 2 {
		t.Fatalf("purge deleted %d (%v), want 2", n, err)
	}
	if got := e.count("SELECT count(*) FROM staff_sessions"); got != 1 {
		t.Fatalf("sessions left %d", got)
	}
	if got := e.count("SELECT count(*) FROM auth_throttle WHERE bucket IN ('old','new')"); got != 1 {
		t.Fatalf("throttle rows left %d", got)
	}
}
