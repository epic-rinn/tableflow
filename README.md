# TableFlow

Restaurant queue, dine-in ordering, cashier settlement, and optional loyalty.

This repository currently contains the MVP knowledge base, development rules, review skills, and empty runtime directories. Application code, database migrations, and runtime tests are not implemented yet.

Start at [specs/README.md](specs/README.md). There are three applications: a Next.js admin panel for restaurant staff, a Next.js customer PWA, and a Go API backed by PostgreSQL. SQL is handwritten through pgx and migrated with Goose. No ORM.

## Working with an AI agent

Read [AGENTS.md](AGENTS.md), select a feature from the [MVP roadmap](specs/delivery/roadmap.md), and use `$tableflow-deliver-feature`. Each feature progresses through specification, plan, tasks, implementation, verification, and review. Use `$tableflow-code-review` for an explicit review and `$tableflow-db-api-review` for query/endpoint analysis.

Repository skills live in `.agents/skills/`. Launch Codex from this folder; restart if newly added skills are not visible. These instructions do not configure a hosted review bot or bypass environment permissions.

## Layout

```text
AGENTS.md                       Shared agent rules and completion gate
.agents/skills/                 Delivery and review procedures
specs/README.md                 Knowledge map and source-of-truth rules
specs/product/                  MVP boundaries, roles, vocabulary
specs/features/                 Testable behavior by capability
specs/architecture/             Components, schema design, security
specs/api/                      HTTP contract and endpoint inventory
specs/quality/                  Test strategy and performance budgets
specs/decisions/                Architecture decision records
specs/delivery/                 Roadmap and development workflow
specs/changes/                  Per-feature plans, tasks, review evidence
specs/templates/                Reusable change and review formats
specs/research/                 External sources and adopted practices
docs/development/               Application setup and engineering guides
tooling/specs/                  Knowledge-base validation (not app code)
src/admin/                      Staff/manager Next.js admin panel
src/pwa/                        Customer Next.js PWA
src/api/                        Go API, SQL, and runtime tests
src/api/db/migrations/          Versioned Goose SQL migrations
```

Run `make specs-check` with Python 3.10+ to check local Markdown links and the required knowledge/skill structure. Runtime setup and commands will be introduced by milestone M0; this repository does not pretend to have a runnable app yet.

`src/` contains runtime projects only. Specs, documentation, agent instructions, skills, and knowledge-base tooling stay outside it and are excluded from the root Docker build context. Each runtime project will own its dependencies/build configuration in M0. See [repository boundaries](specs/architecture/repository.md).
