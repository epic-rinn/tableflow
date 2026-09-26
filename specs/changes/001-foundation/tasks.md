# Tasks: 001-foundation

Status: planned. Owner: Claude. Backlog task: MVP-01.

- [ ] Inspect user changes and read the linked architecture/development guidance.
- [ ] Record pinned compatible versions, package manager, commands and environment assumptions.
- [ ] Bootstrap separate admin/PWA Next.js shells and Go service under the existing source boundaries.
- [ ] Add local PostgreSQL/mail services and secret-free example configuration.
- [ ] Establish distinct frontend origins and explicit Go API routing.
- [ ] Add pgx connection management, Goose migration and real disposable-DB smoke test.
- [ ] Add health/readiness, request IDs, bounded dependency checks and graceful shutdown.
- [ ] Add and validate OpenAPI for the implemented routes only.
- [ ] Introduce actual format/static/unit/race/DB/build/browser-smoke commands and CI jobs.
- [ ] Document clean-checkout setup, migration execution and recovery outside source directories.
- [ ] Run clean-setup, independent builds, dependency-failure and production-artifact boundary checks.
- [ ] Record exact commands/results and limited health endpoint measurements.
- [ ] Perform tableflow-code-review and tableflow-db-api-review as distinct post-implementation passes.
- [ ] Fix findings, rerun relevant checks, and review the final changes again.
- [ ] Complete review.md and update MVP-01 status; leave M0 incomplete until MVP-02–04 are done.

Do not start identity or restaurant features as part of this packet. Never check off unrun verification.
