# Change: 021-operations — Deployment and restore rehearsal

Status: **blocked** on live email only (the user's Resend domain and key); everything else was verified at the M6 gate (2026-09-27). Date: 2026-09-27. Scope owner: Claude. Task: MVP-21. Verification: M6 gate (ADR-0004).

## Problem and behavior

Maps ACC-002/004, PWA-001, the release gates and [repository boundaries](../../architecture/repository.md). Deliver a reproducible staging deployment and runbook, secret handling, HTTPS on separate origins, health monitoring, a migration recovery procedure, a backup/restore drill and email configuration checks. **No production deployment without approval.**

## Decisions

- **Images:** `deploy/api.Dockerfile` (static Go binaries on distroless, non-root, including `migrate`, `tableflowctl` and the migrations) and `deploy/next.Dockerfile` (standalone Next.js as `node`, `APP=admin|pwa`).
- **Staging stack:** `deploy/staging/compose.yaml` runs PostgreSQL, a one-shot migrate job, the API, admin, PWA and Caddy.
  - Caddy terminates HTTPS for two hostnames. `CADDY_TLS=internal` uses a local CA for rehearsal; empty uses ACME.
  - `TRUSTED_PROXY_CIDRS` is limited to the stack's private subnet, so throttling sees real client IPs (Caddy overwrites spoofed `X-Forwarded-For`).
- **Health:** `tableflow-api -healthcheck` probes readiness, for images without curl. `DB_POOL_STATS_INTERVAL` logs pool saturation (added in MVP-20).
- **Backup/restore:** `tooling/runtime/restore-drill.sh` runs `pg_dump -Fc`, restores into a new database and reconciles settlements, snapshot totals, refunds, claims and ledger versus profiles. It also checks the app role and migration version.
- **Runbook:** [docs/operations/deployment.md](../../../docs/operations/deployment.md).
- **Email:** configuration and TLS paths are tested (ADR-0005). Live delivery needs the user's Resend domain and key, so it stays **blocked** here and is recorded as an open decision.

## Verification map

| Requirement | Evidence | Status |
| --- | --- | --- |
| Restored database reconciles bills, ledger and claims | `restore-drill.sh`: E2E database ([evidence](evidence/restore-drill.json)) and the 1M-line perf database ([evidence](evidence/restore-drill-perf.json), 91 MB dump, 10 s dump+restore) | passing |
| Fresh and upgrade migrations | `TestFreshMigrationAndDatabaseSmoke`, `TestUpgradeFromPreviousRelease` (in `make verify`) | passing |
| Staged artifacts exclude specs, docs and AI files | [image-inspection.json](evidence/image-inspection.json) + `make artifact-check` | passing |
| Security, session and cache checks on deployed HTTPS origins | `src/pwa/deploy-checks/origins.spec.ts` on the local HTTPS staging stack ([results](evidence/staging-rehearsal.md)) | passing |
| Production email (ADR-0005) | Live test sends | **blocked**: needs the user's domain and key |

## Implementation notes

- **HSTS:** the first HTTPS probe showed no HSTS header; `Strict-Transport-Security` was added in the Caddyfile.
- **Example env file:** the first `staging.env.example` set `SMTP_USERNAME=resend` without a password, which the API correctly refuses. Both are now empty until the key exists.
- **Deployed-origin checks:** `playwright.deploy.config.ts` runs against any staging URLs. `STAGING_INSECURE_TLS=1` is only for a local CA, because Chrome will not register service workers under a certificate error.
- **Blocked item:** live Resend delivery (verification, reset, activation to authorised recipients; tracking off). Next action: the user provides the verified sending domain and API key through a secret store, then the runbook email steps run.
