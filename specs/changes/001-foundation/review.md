# Review: 001-foundation

Date: 2026-09-26. Reviewer: Claude — **self-review** (implementation author; no independent reviewer ran).
Scope: working tree against `261db41` as reviewed before committing (committed afterwards without code changes), including all untracked files listed by `git status`: `src/api`, `src/admin`, `src/pwa`, `specs/api/openapi.yaml`, `compose.yaml`, `Makefile`, `.github/workflows/runtime.yml`, `tooling/runtime/`, docs/spec updates. Result: **pass with follow-ups**; one P2 remains open with owner and follow-up (below). CI has not executed on GitHub.

Passes performed after implementation and tests: (1) code review per `tableflow-code-review` over the final diff, callers, migrations and tests; (2) DB/API review per `tableflow-db-api-review`, recorded in [performance.md](performance.md). Fixes were re-verified by rerunning the affected tests and the process-level experiment.

## Findings

| Severity | File:line and requirement | Trigger / evidence | User or operational impact | Resolution and recheck |
| --- | --- | --- | --- | --- |
| P2 | `src/api/internal/platform/app/app.go:51` (process lifecycle, GracefulShutdownSmoke) | SIGTERM while PostgreSQL paused: in-flight request drained, but `pgxpool.Close` blocked; process exited after 15.71 s, beyond the 10 s shutdown bound | Slow restarts/deploys; orchestrator SIGKILL instead of clean exit | **Fixed**: `closePool` bounds pool close to 3 s after the drain. `TestClosePoolIsBounded` added; process rerun exited in 4.09 s (paused DB) and 0.02 s (healthy DB, 0 leftover sessions) |
| P2 | `src/admin/next.config.ts`, `src/pwa/next.config.ts` (repository boundary, ADR-0002) | Running `next dev` under an AI agent made Next.js 16.3 write `AGENTS.md` and `CLAUDE.md` into both app roots; `make specs-check` failed | AI instruction files inside runtime source, contrary to the repository boundary | **Fixed**: `agentRules: false` in both configs; generated files deleted; `next dev` rerun did not recreate them; specs-check passes |
| P2 | `src/pwa/proxy.ts:23`, `src/admin/proxy.ts:23` (HTTP contract: 503 on dependency failure) | `API_INTERNAL_URL=http://127.0.0.1:1`, `GET /api/v1/health/live` via frontend → `500 Internal Server Error`, text/plain, no `Cache-Control: no-store` | When the API is down, browsers get a non-contract error body; no data exposure or integrity risk | **Open**. The rewrite cannot intercept upstream connection failures. Owner: Claude. Follow-up: MVP-02 plan must choose between a route-handler proxy (with explicit cookie/header forwarding and a 503 envelope) and a deployment reverse proxy (MVP-21), then add a proxy-failure test |
| P3 | `src/*/app/icon.svg` (AdminAndPwaBrowserSmoke) | Smoke test caught a console `404` for the favicon | Console noise; masked real console errors | **Fixed**: added app icons; smoke rerun clean on both apps |
| P3 | `src/api/internal/platform/health/health.go:65`, `app.go:24` | Readiness is public through both origins; each failed probe logs WARN | Outage floods can amplify logs; up/down state visible to the public | Accepted for MVP-01; revisit with monitoring and deployment routing in MVP-21 |
| P3 | `src/api/db/local/init.sql:8` | `tableflow_app` can still connect to the `postgres` maintenance database (PUBLIC default) | Local/CI only; no production roles exist yet | Accepted; production role hardening belongs to MVP-21 |

Checked and not a finding: encoded path traversal through `proxy.ts` (`/api/v1/%2e%2e/...`, `..%2f`, double encoding) never reached the upstream outside `/api/v1` (upstream request log inspected). Request IDs from clients are accepted only when they match `^[A-Za-z0-9._-]{1,64}$`; access logs omit query strings and path values (tested). Config and connection errors do not echo DSNs/passwords (tested). Test databases are dropped (0 `tableflow_test_*` remaining after runs).

## Verification

All runs 2026-09-26 on macOS (Darwin 25.5.0, arm64), Docker 29.1.3, `postgres:18.6`, Go 1.27.1, Node 24.21.0 (checksum-verified download, used via PATH; system Node is 23), pnpm 12.6.0 via Corepack.

| Command / test | Environment / fixture | Result | Evidence |
| --- | --- | --- | --- |
| `make specs-check` | Python 3 | passed (48, later 49 Markdown files) | Console output |
| `make api-check` (gofmt, `go vet`, `go test -race`) | Go 1.27.1 | passed | DB tests skip here without `TEST_DATABASE_URL` |
| `make api-test-db` (`TABLEFLOW_REQUIRE_DB=1`) | compose PostgreSQL 18.6 | passed, all 6 packages | Includes `TestFreshMigrationAndDatabaseSmoke` (up → down to 0 → up, version 20260926120000), `TestRunAgainstPostgres`, `TestHealthOpenApiValidation` |
| `TestReadinessUnavailableDatabase`, `TestReadinessTimeoutIsBounded`, `TestConcurrentReadinessSharesOneProbe`, `TestReadinessProbeSurvivesCallerCancellation` | Unit + real pgxpool to closed port | passed | 503 in 1.6 ms (refused); 201 ms for 200 ms timeout; 50 callers → 1 probe |
| `TestGracefulShutdownSmoke`, `TestShutdownTimeoutForcesClose`, `TestClosePoolIsBounded` | Unit | passed | In-flight request completes; listener closed |
| Process SIGTERM experiment | Built binary, paused/healthy PostgreSQL | passed after fix | [dependency-and-shutdown.txt](evidence/dependency-and-shutdown.txt) |
| `make migrate-status/up` as `tableflow_owner`; API as `tableflow_app` | Fresh compose volume | passed; app role denied `CREATE TABLE`, has DML (not TRUNCATE) on owner-created tables | psql probes |
| `make admin-check`, `make pwa-check` | Frozen lockfiles | passed independently (ESLint, typegen + tsc, `next build` standalone) | Build output |
| `make smoke` with `PLAYWRIGHT_CHANNEL=chrome` | Google Chrome 153.0.8010.54; API on 8080/8081 | 3/3 passed per app | Bundled Chromium download timed out in this environment, so installed Chrome was used; CI config installs bundled Chromium (not yet run) |
| `pnpm dev` for both apps | API running | `/` 200 on :3000 and :3001; `/api/v1/health/ready` 200 through both | curl |
| `make artifact-check` | Go binary + both standalone/static outputs | passed: 2449 files, 51 sentinels, no leaks; negative control with a copied spec failed as expected | Console output |
| Clean-checkout simulation | Tracked + new non-ignored files copied to scratch; separate compose project on port 54319 | services-up, migrate, specs, api-check, api-test-db, frontends-check, smoke, artifact-check all passed | Console output |
| `actionlint v1.7.7` | Both workflows | passed (static only; shellcheck not installed) | Console output |
| Final rerun after last fix (10:27Z): `make frontends-check`, `make api-check`, `make api-test-db`, `make smoke` (installed Chrome), `make artifact-check`, `make specs-check` | Local compose PostgreSQL 18.6 | all passed; artifact check 2449 files / 52 sentinels | Console output |
| GitHub Actions `runtime.yml` | — | **not run** (requires push, not authorized) | — |

## Database and endpoints

See [performance.md](performance.md): two health routes, one bounded ping per coalesced readiness probe, measured p50/p95/p99 for up/stopped/paused database states, pool/timeouts documented. No domain queries, plans, or load tests apply; this is static review plus limited single-host measurement, not load qualification. Migration: one non-destructive guard migration, tested fresh/down/re-apply; no upgrade path exists yet.

## Delivery decision

Implementation, local verification and both review passes are complete; no P0/P1 findings. Open: P2 proxy error shape (owner Claude, follow-up in MVP-02/MVP-21 decision), P3 items accepted above. Status: **reviewed**, not `done`, because the new CI workflow has never executed. Close to `done` after the first GitHub Actions run of `runtime.yml` passes (needs user-approved push). Canonical specs updated: [repository](../../architecture/repository.md), [system](../../architecture/system.md), [HTTP](../../api/http.md), [OpenAPI](../../api/openapi.yaml), development guides and [setup](../../../docs/development/setup.md).
