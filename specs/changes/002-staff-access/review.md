# Review: 002-staff-access

Date: 2026-09-26. Reviewer: Claude — **self-review** (implementation author; no independent reviewer ran).
Scope: working tree against `d7cb4ef`, including all untracked files: migration `20260926130000_staff_access.sql`, `src/api/internal/identity/**`, `cmd/tableflowctl`, platform changes (config, httpx, app), admin app screens/tests, OpenAPI and spec/doc updates, `tooling/runtime/{verify,e2e-db}.sh`, perf seed. Result: **pass**. The P1 found in review is fixed; one pre-existing P2 (from 001) remains with owner and follow-up.

Passes after implementation and tests: (1) code review per `tableflow-code-review` (Go handlers/services/SQL, middleware, admin UI, tests); (2) DB/API review per `tableflow-db-api-review`, recorded in [performance.md](performance.md). Fixes were re-verified with targeted tests and a final full `make verify`.

## Findings

| Severity | File:line and requirement | Trigger / evidence | Impact | Resolution and recheck |
| --- | --- | --- | --- | --- |
| P1 | `src/api/internal/platform/config/config.go` `TrustedProxies` default; ACC-004 rate limits | Header-echo upstream behind `next start` showed the Next.js proxy forwards a client-supplied `X-Forwarded-For: 6.6.6.6` unchanged (no real hop appended). The default trusted loopback, so Go used the spoofed value | Per-IP login/activation throttles bypassable by rotating a header | **Fixed**: no proxy trusted by default; docs require a trusted proxy that appends/overwrites the header. `TestClientIPTrustsOnlyConfiguredProxies` (8 cases) added; per-IP limit raised to 100/10 min for shared-NAT staff, with the per-email limit (10) protecting accounts |
| P2 | `internal/identity/sql/lock_branch.sql` (lock order) | Mutation test: with the branch lock removed, `TestConcurrentMutualDemotion` (HTTP-level) still passed — it did not reliably overlap | A future regression could reintroduce a real deadlock unnoticed | **Fixed**: deterministic `TestAdminLockOrderPreventsDeadlock` holds both transactions after re-validation; without the lock it reproduces `40P01 deadlock detected`, with it one succeeds and the other gets 401 |
| P2 | `internal/identity/sql/revoke_account_sessions.sql`; DB review | Plan on the seeded fixture: 668 rows / 9,380 buffers for one account (expired sessions re-revoked) | Needless write amplification growing with history | **Fixed**: only unexpired sessions; 2 rows / 62 buffers |
| P2 | Session table growth | No purge of expired sessions | Unbounded table and index growth | **Fixed**: hourly bounded purge of sessions expired >30 days (`TestPurgeRemovesOnlyStaleRows`) |
| P3 | `app/activate/page.tsx` | E2E: opening a second activation link in the same tab (fragment-only navigation) kept the previous "activated" state | Confusing activation for a manager reusing a tab | **Fixed**: re-reads the fragment on `hashchange`; covered by the replay step of the admin E2E |
| P3 | `internal/identity/service.go` SetRoles/Deactivate | Disabled target returned `VERSION_CONFLICT` | Misleading error | **Fixed**: `409 ACCOUNT_DISABLED`, tested |
| P3 | `src/*/playwright.config.ts` | Failed run wrote `error-context.md` under `src/admin/test-results`; specs-check (source boundary) failed | Generated Markdown inside runtime source | **Fixed**: Playwright `outputDir` moved to `tmp/playwright/<app>` |
| P2 (from 001, open) | `src/*/proxy.ts` | API unreachable → Next returns plain 500 | Admin client already shows a generic "temporarily unavailable" message for non-JSON errors | Open; owner Claude; decide with deployment routing in MVP-21 |
| P3 (accepted) | Login under distributed flood | 64 parallel attempts queue behind 4 argon2 slots (max 349 ms) | Legit logins slow during a large distributed attack | Accepted for pilot; revisit with deployment rate limiting (MVP-21) |
| P3 (accepted) | `auth_throttle` | Fixed windows allow up to 2× limit across a boundary; email buckets are unsalted SHA-256 prefixes (kept ≤1 day) | Minor | Accepted; HMAC keying needs the deployment secret introduced later |
| P3 (accepted) | `/login`, `/activate` pages | Next serves these static shells with `s-maxage=31536000` | No private data (token only in fragment); authenticated pages are `private, no-store` (checked with curl) | Accepted |

Checked and not findings: forged/missing Origin and cross-site `Sec-Fetch-Site` rejected on every mutation incl. login/logout/activation; cross-branch access returns 404 before role checks; raw session/activation tokens are never stored (SHA-256 only) or logged; audit rows carry no tokens/passwords; responses are `private, no-store`; unknown/inactive/wrong-password logins are indistinguishable (dummy hash). Playwright's request client omits `Secure` cookies over http, so browser-side authorization checks use in-page `fetch` (the first draft of the E2E would have passed vacuously; corrected before relying on it).

## Verification

All runs 2026-09-26 on macOS arm64, Go 1.27.1, Node 24.21.0, pnpm 12.6.0, Docker 29.1.3, `postgres:18.6`, installed Chrome 153 (`PLAYWRIGHT_CHANNEL=chrome`).

| Command / test | Result | Evidence |
| --- | --- | --- |
| `make verify` (final, 11:42Z) | **passed**: specs-check (54 files), gofmt/vet, race tests, PostgreSQL tests, admin/PWA lint/typecheck/build, browser tests (admin 6/6, PWA 3/3) on a fresh `tableflow_e2e` database, artifact check (2534 files / 57 sentinels); API stopped afterwards | Console output |
| Go tests with `TABLEFLOW_REQUIRE_DB=1` | 45 passed, 0 skipped/failed | Includes TestStaffRoleMatrix, TestCrossBranchStaffAccessIsHidden, TestMutationsRequireAllowedOrigin, TestNoPublicStaffRegistration, TestActivationTokenSingleUse, TestConcurrentActivationOneWinner (8 parallel), TestSessionCookieAttributes, TestAuthenticatedResponsesArePrivate, TestLogoutRevokes, TestSessionExpiry, TestLoginRateLimited, TestActivationRateLimited, TestLastManagerProtected, TestConcurrentMutualDemotion, TestRoleChangeRevokesSessions, TestInFlightMutationAfterRevocationFails, TestRevocationWaitsForInFlightMutation, TestAdminLockOrderPreventsDeadlock, TestStaffAdministrationAudited, TestStaffListPagination, TestRequestValidation, TestPurgeRemovesOnlyStaleRows, TestStaffContractConformance, TestHealthOpenApiValidation, TestFreshMigrationAndDatabaseSmoke, TestUpgradeFromPreviousRelease, TestClientIPTrustsOnlyConfiguredProxies |
| Mutation experiments | Removing `FOR SHARE` fails TestRevocationWaitsForInFlightMutation; removing the branch lock fails TestAdminLockOrderPreventsDeadlock with `40P01`; breaking an OpenAPI field type fails TestStaffContractConformance. All restored | Console output |
| Admin browser tests (`tests/staff.spec.ts`) | passed: activation with fragment removal and replay rejection, cookie attributes, role-aware navigation, kitchen denied `/staff` and `/cashier` in UI and API (403), forged Origin 403, deactivation revokes an open session, sign-out | `make verify` output |
| Proxy header inspection | Next forwards client `X-Forwarded-For` unchanged (basis of the P1) | Header echo |
| Cache headers on admin pages | Authenticated pages and anonymous redirect: `private, no-cache, no-store`; `/login`, `/activate` static | curl |

## Database and endpoints

See [performance.md](performance.md): 9 routes, statement counts, lock order, EXPLAIN ANALYZE of 12 statements on a pessimistic seeded fixture (all hot statements ≤0.26 ms), endpoint p50/p95/p99, argon2 memory bound under a 64-way burst. Migration tested fresh → down → up and upgrade from the previous release. Indicative single-host measurements, not load qualification.

## Delivery decision

No open P0/P1. Open P2 (proxy error shape) carried from 001 with owner/follow-up; accepted P3s listed. Canonical specs updated: [HTTP](../../api/http.md), [OpenAPI](../../api/openapi.yaml), [data model](../../architecture/data-model.md), [access](../../features/05-access-pwa.md) and [admin](../../features/06-admin.md) status, development guides and [setup](../../../docs/development/setup.md). Status: **done** (2026-09-26).
