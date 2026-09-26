# Verification and release gates

## Levels

- Go unit tests: state transitions, money/rounding, tier calculations, validation, and authorization decisions.
- PostgreSQL integration tests: real migrated PostgreSQL, constraints, SQL scanning, branch isolation, rollback, idempotency, and concurrent operations. Do not substitute SQLite or repository mocks for these guarantees.
- API contract tests: request validation, errors, role matrix, resource isolation, pagination, body limits, OpenAPI compatibility, and cancellation.
- Frontend component tests: carts, conflicting/stale data, and disabled offline actions. Browser E2E: join → call → seat → two phones order → kitchen serve → member claim → settle → points → depart → clean.
- PWA browser tests: install prerequisites, service-worker update, forbidden cache contents, reconnection, hidden-tab polling, and anonymous/member switching.
- Application isolation tests: independent admin/PWA builds, staff/customer host-only cookies, PWA service-worker isolation from admin, and no docs/specs/AI files in deployed outputs.
- Migration checks: fresh up and previous-release up on disposable databases, compatibility, and documented rollback/forward-recovery. Backups need a tested restore procedure before launch.
- Performance: representative SQL plans and HTTP load/lock contention per [budgets](performance.md).

## Concurrency tests that must exist

1. Two hosts competing for a table; no-show versus seating; moving versus departure.
2. Same order key/body in parallel; same key/different body; response-loss retry.
3. Sold-out/menu change racing with order acceptance.
4. Order submission or line cancellation racing with begin-settlement.
5. Two cashiers confirming; confirmation retry after response loss; refund replay.
6. Two visits earning for the same member; full refund after policy changes.
7. Token rotation/role revocation while requests are in flight.

Use independent database connections and synchronization barriers; a sequential loop is not a concurrency test. Assert committed database invariants as well as HTTP outcomes. Go's race detector does not detect database races.

## Feature definition of done

- Requirement IDs map to passing named tests; contracts/migrations/docs reflect final behavior.
- Relevant format, static analysis, unit/integration/browser checks ran successfully with exact commands recorded.
- Code review ran after implementation; DB/API review ran where applicable; fixes were re-reviewed.
- Review has no unresolved P0/P1 findings. P2 findings are fixed or carry an explicit rationale, owner, and follow-up; materially broken acceptance behavior cannot be waived as a minor finding.
- Required performance evidence is present, or the feature remains implemented/unverified rather than delivered.
- Operational decisions affecting live service are resolved before launch.

## CI rollout

The current workflow only validates specs/skill structure. M0 must add Go formatting/vet/tests/race checks, pinned frontend lint/typecheck/tests/build, OpenAPI validation, PostgreSQL-backed migration/integration jobs, and browser smoke tests as their code is introduced. Run targeted checks per change; run representative performance jobs for data-path changes, not blindly on every prose edit. Repository branch-protection settings are an external deployment step, not created by Markdown.
