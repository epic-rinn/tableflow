# Tasks: 001-foundation

Status: reviewed (2026-09-26); first CI run pending. Owner: Claude. Backlog task: MVP-01.

- [x] Inspect user changes and read the linked architecture/development guidance.
- [x] Record pinned compatible versions, package manager, commands and environment assumptions.
- [x] Bootstrap separate admin/PWA Next.js shells and Go service under the existing source boundaries.
- [x] Add local PostgreSQL/mail services and secret-free example configuration.
- [x] Establish distinct frontend origins and explicit Go API routing.
- [x] Add pgx connection management, Goose migration and real disposable-DB smoke test.
- [x] Add health/readiness, request IDs, bounded dependency checks and graceful shutdown.
- [x] Add and validate OpenAPI for the implemented routes only.
- [x] Introduce actual format/static/unit/race/DB/build/browser-smoke commands and CI jobs. (Commands run locally; `runtime.yml` statically linted but not yet executed on GitHub.)
- [x] Document clean-checkout setup, migration execution and recovery outside source directories.
- [x] Run clean-setup, independent builds, dependency-failure and production-artifact boundary checks.
- [x] Record exact commands/results and limited health endpoint measurements.
- [x] Perform tableflow-code-review and tableflow-db-api-review as distinct post-implementation passes.
- [x] Fix findings, rerun relevant checks, and review the final changes again.
- [x] Complete review.md and update MVP-01 status; leave M0 incomplete until MVP-02–04 are done.
- [ ] First GitHub Actions run of `runtime.yml` passes (requires approved push), then mark MVP-01 `done`.

Do not start identity or restaurant features as part of this packet. Never check off unrun verification.
