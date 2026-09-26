# Change: 001-foundation

Status: planned. Date: 2026-09-26. Implementation owner: Claude. Task: MVP-01.

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
| Separate runtime boundaries; ADM-001 prerequisite | IndependentAdminAndPwaBuild | Record commands/results in review.md | planned |
| PostgreSQL/Goose foundation | FreshMigrationAndDatabaseSmoke | Disposable DB logs and versions | planned |
| Dependency health | ReadinessUnavailableDatabase | HTTP result and timeout evidence | planned |
| Process lifecycle | GracefulShutdownSmoke | Actual result and connection cleanup | planned |
| No docs/AI in runtime artifacts | ProductionArtifactBoundaryCheck | Artifact listing/check output | planned |
| HTTP contract | HealthOpenApiValidation | Validator output | planned |
| Runnable UI shells | AdminAndPwaBrowserSmoke | Browser test result | planned |

## Final decisions

No implementation decisions or test results recorded yet. Claude must fill in selected versions, actual commands/paths, origin routing and recovery decisions before marking this task complete.
