# Tasks: 001-foundation

Status: done (2026-09-26). Owner: Claude. Backlog task: MVP-01.

- [x] Inspect user changes and read the linked architecture/development guidance.
- [x] Record pinned compatible versions, package manager, commands and environment assumptions.
- [x] Bootstrap separate admin/PWA Next.js shells and Go service under the existing source boundaries.
- [x] Add local PostgreSQL/mail services and secret-free example configuration.
- [x] Establish distinct frontend origins and explicit Go API routing.
- [x] Add pgx connection management, Goose migration and real disposable-DB smoke test.
- [x] Add health/readiness, request IDs, bounded dependency checks and graceful shutdown.
- [x] Add and validate OpenAPI for the implemented routes only.
- [x] Introduce actual format/static/unit/race/DB/build/browser-smoke commands and a verification gate: `make verify` (hosted CI removed per ADR-0003).
- [x] Document clean-checkout setup, migration execution and recovery outside source directories.
- [x] Run clean-setup, independent builds, dependency-failure and production-artifact boundary checks.
- [x] Record exact commands/results and limited health endpoint measurements.
- [x] Perform tableflow-code-review and tableflow-db-api-review as distinct post-implementation passes.
- [x] Fix findings, rerun relevant checks, and review the final changes again.
- [x] Complete review.md and update MVP-01 status; leave M0 incomplete until MVP-02–04 are done.
- [x] Full `make verify` run passes; MVP-01 marked `done` (replaces the hosted CI gate per ADR-0003).

Do not start identity or restaurant features as part of this packet. Never check off unrun verification.
