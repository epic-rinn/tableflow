# Change: 001-foundation

Status: reviewed (CI first run pending). Date: 2026-09-26. Implementation owner: Claude. Task: MVP-01.

## Problem and behavior

The repository contains desired behavior and empty runtime projects, not a runnable restaurant system. Establish a reproducible development and verification foundation. This supports ACC-004 and ADM-001 but does not complete authentication or any domain requirement.

In scope: separate Next.js admin and customer PWA shells; Go service; pgx/PostgreSQL and Goose; pinned compatible versions; local services including a development mail sink; documented configuration/origin routing; health/readiness; minimal OpenAPI and CI/test harness. Out of scope: login, service-worker caching, queue, seating, ordering, settlement and loyalty. Do not expose unfinished domain endpoints.

Read [system](../../architecture/system.md), [repository](../../architecture/repository.md), [HTTP](../../api/http.md), [security](../../architecture/security.md), and the relevant [development guides](../../../docs/development/api.md). Check current official documentation when selecting tool versions; record actual choices without silently changing accepted architecture.

## Implementation plan

1. Inspect current files and identify compatible pinned Go/Node/Next.js/PostgreSQL/Goose versions and one frontend package manager. Record unresolved deployment decisions separately.
2. Bootstrap `src/admin`, `src/pwa`, and `src/api` with independent builds. Go owns API responses; frontends must not implement domain services. Place setup/runbooks outside source.
3. Add disposable local PostgreSQL/mail services, example configuration without secrets, and consistent customer/admin origin routing to Go. Use host-only cookies later; do not introduce shared browser token storage now.
4. Add a minimal Goose migration and real DB harness; define migration ownership and fresh/upgrade/recovery commands. Avoid speculative implementation of the entire domain schema.
5. Introduce liveness independent of DB health, bounded readiness dependency checks, request IDs, graceful shutdown and safe error/logging behavior.
6. Put the initial machine-readable OpenAPI contract at the location required by the system spec, covering only implemented health routes. Add validation and reproducible generation if needed; never import specs at runtime.
7. Add documented formatting/static/unit/DB/frontend/browser-smoke checks and CI jobs as runnable components become available. Verify deployed build contents exclude docs/specs/AI files.

## Query and endpoint impact

Only health/readiness and infrastructure database access are in scope. Record exact routes, timeout values, pool configuration and bounded readiness query count before implementation. Measure health behavior with and without PostgreSQL; review resource cleanup and cancellation. Domain-load budgets are not applicable yet; no claim of restaurant-load qualification is allowed.

## Verification map

| Requirement / boundary | Named test or experiment | Evidence | Status |
| --- | --- | --- | --- |
| Separate runtime boundaries; ADM-001 prerequisite | IndependentAdminAndPwaBuild | `make admin-check`, `make pwa-check` from a clean copy | passed |
| PostgreSQL/Goose foundation | FreshMigrationAndDatabaseSmoke | `dbtest/migrations_test.go` on PostgreSQL 18.6 | passed |
| Dependency health | ReadinessUnavailableDatabase | `health_test.go` + stopped/paused DB runs | passed |
| Process lifecycle | GracefulShutdownSmoke | `server_test.go`, `TestClosePoolIsBounded`, SIGTERM runs | passed after fix |
| No docs/AI in runtime artifacts | ProductionArtifactBoundaryCheck | `make artifact-check` + negative control | passed |
| HTTP contract | HealthOpenApiValidation | `app_test.go` (kin-openapi, OpenAPI 3.1) | passed |
| Runnable UI shells | AdminAndPwaBrowserSmoke | `src/*/tests/smoke.spec.ts` (installed Chrome 153 locally) | passed |

## Pre-implementation decisions (recorded 2026-09-26)

Versions were selected from the official registries/proxies on 2026-09-26: Go 1.27.1 toolchain (module minimum `go 1.27`), pgx v5.11.0, Goose v3.28.0 (library and `go tool goose`, pinned by `go.mod`), Node.js 24.21.0 LTS, pnpm 12.6.0 via `packageManager`, Next.js 16.3.6, React 19.3.0, TypeScript 6.0.3 (TypeScript 7 is not assumed compatible with the Next.js type-check API), ESLint 9.39.5 with `eslint-config-next` 16.3.6, Playwright 1.63.0, PostgreSQL image `postgres:18.6`, Mailpit `axllent/mailpit:v1.31.2`. Assumption (reversible): TypeScript/ESLint majors can be raised once Next.js documents support.

Routes (Go, also reachable through each frontend's `/api/v1` proxy):

- `GET /api/v1/health/live` → `200 {"status":"ok"}`; never touches PostgreSQL.
- `GET /api/v1/health/ready` → `200 {"status":"ready","checks":{"database":"ok"}}` or `503` with the common error shape (`DEPENDENCY_UNAVAILABLE`). Exactly one database round trip (pgx ping) per probe; concurrent probes share one in-flight check so readiness can hold at most one pool connection. Probe timeout 1 s (`READINESS_TIMEOUT`).
- Both return `Cache-Control: no-store` and `X-Request-ID`. Other paths return JSON `404 NOT_FOUND`/`405 METHOD_NOT_ALLOWED`.

Pool and timeouts: `pgxpool` with `DB_MAX_CONNS` default 10, no minimum connections (the API starts while PostgreSQL is down and reports not-ready), connection `statement_timeout` 5 s, `lock_timeout` 2 s, `idle_in_transaction_session_timeout` 10 s, `application_name=tableflow-api`. HTTP server: read-header 5 s, read 10 s, write 15 s, idle 60 s, 64 KiB max headers; graceful shutdown on SIGINT/SIGTERM with a 10 s drain (`SHUTDOWN_TIMEOUT`) and pool close after the listener drains.

Request IDs: a client/proxy `X-Request-ID` is reused only if it is 1–64 characters of `[A-Za-z0-9._-]`; otherwise a random 128-bit hex ID is generated. Access logs are JSON (`log/slog`) with method, route pattern, status, bytes, duration and request ID; query strings are not logged because later capability tokens must never reach logs.

Origins: PWA `http://localhost:3000`, admin `http://localhost:3001`, API `http://localhost:8080`. Each Next.js app rewrites `/api/v1/:path*` to its server-only `API_INTERNAL_URL`. No CORS is enabled on Go. Production HTTPS origin routing is a deployment decision for MVP-21.

Database roles: local compose creates an owner/migration role (`tableflow_owner`) and a restricted runtime role (`tableflow_app`) with default DML privileges on objects created by the owner. The first migration only asserts the PostgreSQL 18 baseline; no domain schema is created in MVP-01.

## Final decisions

- Versions, routes, pool/timeouts, request IDs and roles are as recorded above. Changes during implementation: pool close after drain is bounded to 3 s (verification finding); `@types/node` pinned to 24.13.6 because pnpm 12's minimum-release-age policy rejected the day-old 24.19.0; kin-openapi v0.149.0 added as a test-only dependency (not linked into the binary).
- Goose runs through `src/api/cmd/migrate` (PostgreSQL-only provider) instead of the upstream CLI, which would add every Goose database driver to the module graph. Commands: `make migrate-status|migrate-up|migrate-down`.
- Origin routing uses Next.js 16 `proxy.ts` rewrites resolved at request time. Open P2: upstream connection failure returns Next's plain 500 instead of the 503 envelope; to be decided in MVP-02/MVP-21 (see [review](review.md)).
- Local services: `compose.yaml` (PostgreSQL 54318, Mailpit 1025/8025). Setup and recovery commands: [setup](../../../docs/development/setup.md).
- The Playwright load runner (k6) and representative performance harness are not selected yet; no domain path exists.
- Recovery: migrations are a release step; the baseline has no objects to reverse. Fresh → down → re-apply is tested; upgrade tests begin with the second migration.
