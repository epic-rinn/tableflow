# Change: 002-staff-access — Staff identity and access

Status: done. Date: 2026-09-26. Scope owner: Claude. Task: MVP-02.

## Problem and behavior

Maps ACC-002 (staff part), ACC-003, ACC-004, ADM-001 and the staff-account subset of OPS-001 in [access](../../features/05-access-pwa.md) and [admin](../../features/06-admin.md); controls from [security](../../architecture/security.md). Before: no identity. After: an operator bootstraps one branch and its first manager from the command line; the manager invites staff, assigns roles, reissues activation links and deactivates accounts in the admin app; staff activate with a single-use link, sign in and out, and see only the workspaces their roles permit. Go enforces branch, role, session, Origin and rate-limit checks on every call.

In scope: branches, staff accounts, roles, activation tokens, staff sessions, authentication throttling, audit events for staff administration, admin login/activation/home/staff screens. Out of scope: guest/member identity (MVP-03/04), business idempotency keys (MVP-03 infrastructure; staff-admin writes use natural uniqueness and `expected_version` instead), email delivery of invitations (the manager shares the one-time link; email adapter arrives in MVP-04), workspaces other than staff management (placeholders only).

## Decisions and assumptions (reversible unless noted)

- **Separate staff tables.** Staff credentials live in `staff_accounts`/`staff_sessions`, not a shared `accounts` table with members. A self-registered member credential can never acquire staff authority, and manager provisioning stays independent. The logical data model is updated accordingly.
- **One branch per staff account** (the pilot has one branch); roles are branch-scoped through the account's branch. Multi-branch staff would add a mapping table later.
- **Roles:** `host` (host/server), `kitchen`, `cashier`, `manager`; any non-empty combination.
- **Bootstrap:** `go run ./cmd/tableflowctl bootstrap-branch -name ... -email ...` with the app role creates the first branch and an *invited* manager, and prints a one-time activation token. It refuses when any branch already exists. No password is passed on a command line.
- **Activation:** manager-issued token (256-bit random, SHA-256 stored), valid 72 h, single use, replaced when reissued. The link is `/activate#<token>`; the admin page removes the fragment from history and POSTs the token.
- **Passwords:** argon2id (m=19 MiB, t=2, p=1; OWASP minimum), PHC-encoded, 12–128 characters, no composition rules. At most 4 concurrent hashes per process. Unknown emails hash against a dummy to equalise timing.
- **Sessions:** 256-bit random token in cookie `__Host-tf_staff` (HttpOnly, Secure, SameSite=Strict, Path=/, host-only). Only the SHA-256 hash is stored. Absolute lifetime 12 h, idle timeout 60 min; `last_seen_at` is written at most once per minute. Logout, role change and deactivation revoke sessions in the same transaction.
- **In-flight revocation:** every mutation re-validates the actor's session, account status and roles inside its transaction with `FOR SHARE`; revocation takes `FOR UPDATE` on the same rows, so a mutation either commits before the revocation or fails.
- **CSRF:** every non-GET request, including login and activation, requires an `Origin` in `ADMIN_ORIGINS`; `Sec-Fetch-Site`, when sent, must be `same-origin`. SameSite=Strict is defence in depth.
- **Rate limits:** fixed windows in PostgreSQL (`auth_throttle`) so limits hold across replicas: login 10 per email-hash and 100 per client IP per 10 min (raised from 30 during review); activation 20 per IP per 10 min. Client IP comes from `RemoteAddr` unless it is in `TRUSTED_PROXY_CIDRS` (default loopback), in which case the right-most untrusted `X-Forwarded-For` entry is used. **Changed in review:** no proxy is trusted by default, because the Next.js proxy forwards client-supplied `X-Forwarded-For` unchanged. Old buckets are purged hourly, as are sessions expired more than 30 days.
- **Existence hiding:** a resource in another branch returns 404 before any role check; wrong role in one's own branch returns 403.
- **Last manager:** changes that would leave a branch without an active manager return 409 `LAST_MANAGER`; role changes lock the branch row first, which serialises concurrent demotions.
- **Frontend proxy P2 from 001:** the Next.js rewrite forwards cookies and `Set-Cookie` unchanged, which this task relies on. The plain-500 response when the API is down remains a P2 until MVP-21 chooses the deployment routing.

## Implementation plan

1. Migration `…_staff_access.sql`: `branches`, `staff_accounts`, `staff_roles`, `staff_activation_tokens`, `staff_sessions`, `auth_throttle`, `audit_events`, with checks, foreign keys and the indexes listed below.
2. `internal/identity`: password hashing, tokens, cookie handling, session authentication middleware, Origin guard, throttle, handlers, service transactions, embedded SQL.
3. `internal/platform`: JSON body decoding (16 KiB limit, unknown fields rejected), client-IP resolution, `Cache-Control: private, no-store` for authenticated responses, config for origins, trusted proxies and lifetimes.
4. `cmd/tableflowctl bootstrap-branch`.
5. OpenAPI routes and contract tests; `http.md` gains activation, current-session and staff-lifecycle routes.
6. Admin UI: `/login`, `/activate`, role-aware home navigation, `/staff` management; server-side session read through `API_INTERNAL_URL`.
7. `make verify` runs a disposable E2E database with a bootstrapped branch for browser tests.

## Routes

| Route | Actor | Result |
| --- | --- | --- |
| POST `/sessions/staff` | Public, throttled, Origin | 201 staff identity + cookie; 401 `INVALID_CREDENTIALS`; 429 |
| GET `/sessions/current` | Staff session | 200 identity, branch, roles |
| DELETE `/sessions/current` | Staff session, Origin | 204, cookie cleared |
| POST `/staff/activate` | Token, throttled, Origin | 200; 422 `TOKEN_INVALID` (invalid, used or expired alike) |
| GET `/branches/{branch_id}/staff` | Manager of branch | Cursor page (limit ≤100) |
| POST `/branches/{branch_id}/staff` | Manager | 201 invited account + one-time activation token; 409 `EMAIL_TAKEN` |
| POST `/staff/{staff_id}/activation` | Manager | 201 new token for an invited account (old token invalidated) |
| PATCH `/staff/{staff_id}/roles` | Manager | `{expected_version, roles}` → 200; revokes the target's sessions |
| POST `/staff/{staff_id}/deactivate` | Manager | `{expected_version, reason}` → 200; revokes sessions |

## Query and endpoint impact

- Authentication: one statement per request (session ⋈ account with aggregated roles) by unique `token_hash`; one throttled `last_seen_at` update at most per minute per session.
- Staff list: `(branch_id, created_at, id)` keyset over a branch; roles aggregated in the same statement. Pilot cardinality is under 100 staff per branch.
- Mutations: short transactions — actor re-validation (`FOR SHARE`), branch/target locks, writes, audit insert. No external calls in transactions.
- Indexes: unique `lower(email)`, unique `token_hash` on sessions and tokens, `staff_sessions(staff_account_id) WHERE revoked_at IS NULL` for revocation, `staff_accounts(branch_id, created_at, id)`, `audit_events(branch_id, occurred_at, id)`, `auth_throttle(window_start)` for purge.
- Measurement: EXPLAIN ANALYZE for the session lookup and staff list on seeded data, plus a login/current-session latency sample. Record results in `performance.md`.

## Verification map

| Requirement / scenario | Named test or experiment | Evidence location | Status |
| --- | --- | --- | --- |
| ACC-003 role matrix | TestStaffRoleMatrix | review.md | passed |
| ACC-A1 cross-branch | TestCrossBranchStaffAccessIsHidden | review.md | passed |
| ACC-A3 forged Origin | TestMutationsRequireAllowedOrigin | review.md | passed |
| ACC-A3 revocation incl. in-flight | TestRoleChangeRevokesSessions, TestInFlightMutationAfterRevocationFails, TestRevocationWaitsForInFlightMutation | review.md | passed |
| ACC-002 no self-registration; single-use activation | TestNoPublicStaffRegistration, TestActivationTokenSingleUse, TestConcurrentActivationOneWinner | review.md | passed |
| ACC-004 expiry, logout, cookie attributes, private headers | TestSessionExpiry, TestLogoutRevokes, TestSessionCookieAttributes, TestAuthenticatedResponsesArePrivate | review.md | passed |
| ACC-004 rate limits | TestLoginRateLimited, TestActivationRateLimited | review.md | passed |
| Last manager safety | TestLastManagerProtected, TestConcurrentMutualDemotion | review.md | passed |
| OPS-001 audit | TestStaffAdministrationAudited (no secrets) | review.md | passed |
| ADM-001/ADM-A1 | Admin browser test: activate, login, role-aware nav, kitchen denied staff screen and API | review.md | passed |
| Contract | TestOpenApiValidation (all implemented routes) | review.md | passed |
| Performance | Session lookup and staff list plans; login/current latency | performance.md | measured |

## Final decisions

- Accepted as planned: separate staff tables, one branch per staff account, argon2id parameters, cookie attributes and lifetimes, Origin/Sec-Fetch-Site CSRF guard, branch-first lock order, 404-before-403 existence hiding, last-manager rule.
- Changed during review: no trusted proxies by default (P1), per-IP limit 100, revocation of unexpired sessions only, 30-day session purge, `ACCOUNT_DISABLED` error code, activation page re-reads the fragment on `hashchange`.
- `GET/DELETE /sessions/current` serve staff sessions only; MVP-03/04 extend them to guest/member cookies on the PWA origin.
- No ADR needed: decisions stay within ADR-0001/0002. The data model records the staff/member credential split and the staff-admin lock order.
