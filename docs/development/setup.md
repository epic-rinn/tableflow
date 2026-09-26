# Local setup and verification

Introduced by change [001-foundation](../../specs/changes/001-foundation/plan.md). Commands run from the repository root unless stated.

## Toolchain

| Tool | Version | Pinned by |
| --- | --- | --- |
| Go | 1.27.1 | `src/api/go.mod` `toolchain` (older local Go downloads it automatically with the default `GOTOOLCHAIN=auto`) |
| pgx / Goose | v5.11.0 / v3.28.0 | `src/api/go.mod`, `go.sum` |
| Node.js | 24.21.0 LTS | `src/*/.node-version`, `engines` |
| pnpm | 12.6.0 | `packageManager` in each frontend; run through `corepack pnpm` |
| Next.js / React | 16.3.6 / 19.3.0 | Each frontend's `package.json` and `pnpm-lock.yaml` |
| PostgreSQL | `postgres:18.6` | `compose.yaml` |

Both `next.config.ts` files set `agentRules: false`; without it `next dev` run by an AI agent writes `AGENTS.md`/`CLAUDE.md` into the app, violating the source boundary. Also required: Docker with Compose, Python 3.10+ (`make specs-check`, `make artifact-check`). Each frontend has its own `pnpm-workspace.yaml` that denies the `unrs-resolver` install script (its native bindings ship prebuilt); review any new `allowBuilds` prompt instead of approving it blindly.

## First run from a clean checkout

```sh
make services-up        # PostgreSQL on 127.0.0.1:54318 (no local mail sink; email uses Resend)
make migrate-up         # release-step migrations as tableflow_owner
make api-run            # Go API on http://127.0.0.1:8080 as tableflow_app (separate terminal)
cd src/pwa && corepack pnpm install --frozen-lockfile && corepack pnpm dev     # http://localhost:3000
cd src/admin && corepack pnpm install --frozen-lockfile && corepack pnpm dev   # http://localhost:3001
```

The first time, create the branch and its first manager (refuses once any branch exists), then open the printed link on the admin origin:

```sh
cd src/api && DATABASE_URL='postgres://tableflow_app:app_dev_only@127.0.0.1:54318/tableflow?sslmode=disable' \
  go run ./cmd/tableflowctl bootstrap-branch -name "Main" -email manager@example.com -display-name "Manager"
# open http://localhost:3001/activate#<activation_token>
```

`src/api/db/local/init.sql` runs only when the PostgreSQL volume is first created. It creates the owner/migration role `tableflow_owner`, the restricted runtime role `tableflow_app` (DML on owner-created objects, no DDL), and the `tableflow` database. All passwords are dummy local values; `make services-reset` deletes the volume.

## Origins and routing

| Origin (local) | Application | API access |
| --- | --- | --- |
| `http://localhost:3000` | Customer PWA (`src/pwa`) | Same-origin `/api/v1/*` |
| `http://localhost:3001` | Staff admin (`src/admin`) | Same-origin `/api/v1/*` |
| `http://127.0.0.1:8080` | Go API (`src/api`) | Direct; not called by browsers |

Each frontend's `proxy.ts` rewrites `/api/v1/*` to the server-only `API_INTERNAL_URL` at request time (see each app's `.env.example`), so one build can target any API host and no CORS is enabled. Production HTTPS hosts and routing are decided in MVP-21.

## API configuration

See `src/api/.env.example`. `DATABASE_URL` is required. Pool/timeouts: `DB_MAX_CONNS` (10), `DB_STATEMENT_TIMEOUT` (5s), `DB_LOCK_TIMEOUT` (2s), `DB_IDLE_IN_TRANSACTION_TIMEOUT` (10s), `READINESS_TIMEOUT` (1s), `SHUTDOWN_TIMEOUT` (10s), plus `HTTP_*` server timeouts. Guest/member: `PWA_ORIGINS` (default `http://localhost:3000,http://127.0.0.1:3000`), `PWA_PUBLIC_URL` (links in emails), `SMTP_ADDR` (default `smtp.resend.com:587`), `SMTP_TLS` (`starttls` default or `implicit`; plaintext is not supported and STARTTLS never falls back), `SMTP_USERNAME=resend` and `SMTP_PASSWORD` (the Resend API key; export it in your shell, never commit it), `MAIL_FROM` (an address on the verified sending domain, pending). Local development uses Resend too ([ADR-0005](../../specs/decisions/0005-production-email.md)); without credentials the API logs "mail delivery not configured" and sends fail. `MAIL_ADAPTER=file` with `MAIL_OUTBOX_DIR` is a test-only outbox used by `make verify`, and the required `DATA_ENCRYPTION_KEY` (base64 of 32 bytes; the Makefile supplies a dummy local value). Staff identity: `ADMIN_ORIGINS` (exact origins allowed to send staff mutations; default `http://localhost:3001,http://127.0.0.1:3001`), `STAFF_SESSION_IDLE` (60m), `STAFF_SESSION_ABSOLUTE` (12h), `TRUSTED_PROXY_CIDRS` (default none). Only list a reverse proxy that appends or overwrites `X-Forwarded-For` with the real client address; the Next.js dev proxy passes client-supplied values through and must not be trusted. The API does not load `.env` files; the Makefile or the deployment environment supplies values.

Health: `GET /api/v1/health/live` never touches PostgreSQL; `GET /api/v1/health/ready` returns 503 `DEPENDENCY_UNAVAILABLE` when PostgreSQL is unreachable or slower than the readiness timeout. The wire contract is [openapi.yaml](../../specs/api/openapi.yaml).

## Checks

| Command | What it runs |
| --- | --- |
| `make specs-check` | Knowledge-base links/structure |
| `make api-check` | `gofmt`, `go vet`, `go test -race` (PostgreSQL tests skip without `TEST_DATABASE_URL`) |
| `make api-test-db` | All Go tests against the compose PostgreSQL; creates and drops `tableflow_test_*` databases; fails instead of skipping |
| `make admin-check`, `make pwa-check` | Frozen install, ESLint, route typegen + `tsc`, production build, each app independently |
| `make smoke` | Playwright browser tests for both built apps; requires a running API. The admin staff-access tests also need `E2E_MANAGER_TOKEN` from `tooling/runtime/e2e-db.sh` (a fresh `tableflow_e2e` database), which `make verify` provides; otherwise they are skipped. Set `PLAYWRIGHT_CHANNEL=chrome` to use an installed Chrome if the bundled Chromium cannot be downloaded |
| `make artifact-check` | Builds the API binary and scans it plus both Next.js standalone/static outputs for docs/specs/AI content |

`make verify` runs all of the above in order, starting the built API for the smoke tests and stopping it afterwards. It is the required gate; there is no hosted CI ([ADR-0003](../../specs/decisions/0003-local-verification.md)). It requires Node.js 24 on `PATH`.

## Migrations and recovery

`make migrate-status`, `make migrate-up`, `make migrate-down` wrap `go run ./cmd/migrate` (PostgreSQL-only Goose provider, pinned library) using `MIGRATION_DATABASE_URL`. Migrations are a release step; the API never migrates on startup. See [database](database.md) and [migration authoring](migrations.md) for fresh/upgrade testing and recovery rules.
