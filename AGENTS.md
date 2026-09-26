# TableFlow agent instructions

## Start here

- Read `specs/README.md`, then only the feature, architecture, and quality documents relevant to the task.
- Treat `specs/features/` as desired MVP behavior, not evidence that code exists. Read `specs/delivery/roadmap.md` for implementation status.
- Follow explicit user instructions first. Resolve contradictions in the same change; record material architecture changes in an ADR.
- Keep the source of each rule in one place and link to it. Preserve `specs/idea.md` as research history.
- Runtime projects are `src/admin`, `src/pwa`, and `src/api`. Keep documentation in `docs/`, requirements in `specs/`, skills in `.agents/`, and knowledge tooling in `tooling/`; do not put these or AGENTS.md files inside `src/`.
- Current division of work: Codex primarily prepares tasks/specs and updates the original Word report; Claude implements assigned tasks from `specs/delivery/tasks.md` and records tests/reviews/status. Follow a later explicit user assignment if it changes this division. Claude starts at root `CLAUDE.md`; do not assume another agent has been launched merely because a task packet exists.
- For source work read the relevant guide in `docs/development/`: `admin.md`, `pwa.md`, `api.md`, or `database.md`. Those guides replace scoped instructions inside source directories.

## Delivery

- For feature work, follow `.agents/skills/tableflow-deliver-feature/SKILL.md`.
- Before implementation, create or update `specs/changes/<change-id>/plan.md` and `tasks.md` using the templates. Tie tests to requirement IDs. Routine fixes can use a short change record.
- Next.js owns presentation/PWA behavior; Go owns authorization and business rules; PostgreSQL owns durable state. Use handwritten parameterized SQL through pgx, and Goose SQL migrations. No ORM or query builder.
- Do not claim unrun checks passed. Do not mark a feature complete with unresolved blocking findings or missing required runtime evidence.
- Update relevant specs, contracts, and tasks with the code. Never silently weaken acceptance criteria to match implementation.

## Code Review Rules

- Reviews run at each milestone gate ([ADR-0004](specs/decisions/0004-milestone-verification.md)). After implementation and tests, and before reporting a feature delivered, run `.agents/skills/tableflow-code-review/SKILL.md`. Re-review fixes.
- If an endpoint, SQL query, schema, transaction, polling flow, or data-fetching path changed, also run `.agents/skills/tableflow-db-api-review/SKILL.md` and record evidence.
- Review the final diff plus callers, permissions, migrations, and tests. Flag concrete correctness, security, reliability, and measured performance issues with file/line and reproduction conditions.
- Block delivery on cross-visit access, double seating, duplicate orders/payments/points, unverified payment closure, destructive migration risk, and demonstrated serious regressions.
- Do not prescribe indexes or caching from intuition alone. Explain the query shape, cardinality, plan, tradeoff, and measurement limitations.
- These are agent workflow requirements. `make specs-check` checks structure, not review quality; there is no hosted CI or hosted review ([ADR-0003](specs/decisions/0003-local-verification.md)); `make verify` is the local verification gate.

## Current checks

- `make verify` runs the full local gate (see [setup](docs/development/setup.md)); `make specs-check` checks the knowledge base only.
- App, Go, database, and load-test commands must be introduced and documented in M0. Do not run imaginary package scripts.
- Use disposable local databases for migrations, concurrency tests, and `EXPLAIN ANALYZE`. Never run write plans or load tests on production without explicit authorization.
