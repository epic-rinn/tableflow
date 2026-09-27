# Deployment and operations runbook (MVP-21)

Status: a staging rehearsal is defined and exercised locally (see [021 evidence](../../specs/changes/021-operations/plan.md)). **No production deployment is authorised**; the hosting provider, public hostnames, backup storage and Resend domain/key are open decisions (last section).

## Topology

```
browser ──HTTPS──▶ Caddy ─┬─▶ admin (Next.js standalone) ─┐
                          └─▶ pwa   (Next.js standalone) ─┴─▶ api (Go) ──▶ PostgreSQL 18
```

- **Origins:** two hostnames, admin and PWA. Each Next.js server forwards `/api/v1/*` to `API_INTERNAL_URL`, so browsers stay same-origin with no CORS. The API is never exposed publicly.
- **Scale:** one API replica with `DB_MAX_CONNS=10` (the default). Size the pool against PostgreSQL `max_connections` before adding replicas.
- **Images:** built from the repository root.
  - `deploy/api.Dockerfile`: distroless, non-root; includes `migrate` and `tableflowctl`.
  - `deploy/next.Dockerfile`: `--build-arg APP=admin|pwa`; runs as `node`.
  - The root `.dockerignore` keeps specs, docs, tooling and AI files out of the build context, and `make artifact-check` scans the built outputs.
- **Staging stack:** `deploy/staging/compose.yaml` with `Caddyfile`, `init-roles.sh` and `staging.env.example`.

## Secrets and configuration

Supply configuration as environment variables from the platform's secret store. The local file `deploy/staging/staging.env` is git-ignored. Never commit values, and never put secrets in `NEXT_PUBLIC_*` variables.

| Variable | Where | Notes |
| --- | --- | --- |
| `DATABASE_URL` | api | App role `tableflow_app` (DML only) |
| `MIGRATION_DATABASE_URL` | migrate job | Owner role `tableflow_owner`; used only for migrations |
| `DATA_ENCRYPTION_KEY` | api | 32 random bytes, base64. Seals idempotency replays; losing it only invalidates replays, which expire within days |
| `ADMIN_ORIGINS`, `PWA_ORIGINS`, `PWA_PUBLIC_URL` | api | Exact `https://` origins; the CSRF origin guard uses them |
| `TRUSTED_PROXY_CIDRS` | api | Only the private network of Caddy and the Next servers (see below). Never `0.0.0.0/0` |
| `SMTP_USERNAME=resend`, `SMTP_PASSWORD` (Resend API key), `MAIL_FROM` | api | ADR-0005; STARTTLS on 587 is required. Missing credentials log an error at startup and every send fails |
| `API_INTERNAL_URL` | admin, pwa | Server-only, e.g. `http://api:8080` |
| `NEXT_PUBLIC_PWA_URL` | admin build arg | The PWA origin, for dining and tracking links |
| `DB_POOL_STATS_INTERVAL` | api (optional) | e.g. `1m`; logs pool saturation |

**Client IPs and throttling:** Caddy replaces a client-supplied `X-Forwarded-For` with the real client address, and the Next servers pass it on. With `TRUSTED_PROXY_CIDRS` set to that private network, the API throttles per real client. Without it, every guest shares the proxy's address and one per-IP bucket.

## First deployment

1. Provision PostgreSQL 18 and create the roles with `init-roles.sh`, or the platform equivalent. Only the app role is given to the API.
2. Run migrations as a one-shot job: `migrate up` (the compose `migrate` service). The API starts only after migrations succeed.
3. Create the branch and first manager: `tableflowctl bootstrap-branch -name "<Branch>" -email <manager email> -display-name "<Name>"`, with `DATABASE_URL` set. It prints a one-time activation token; send the manager `https://<admin host>/activate#<token>` privately.
4. Run the deployed-origin checks from `src/pwa`. They check HTTPS and HSTS, host-only Secure cookies, private no-store responses, the origin guard, that the admin has no service worker, and the PWA's public-only caches:
   `STAGING_ADMIN=https://<admin host> STAGING_PWA=https://<pwa host> STAGING_BRANCH_ID=<id> STAGING_ACTIVATION_TOKEN=<one-time token> pnpm exec playwright test -c playwright.deploy.config.ts`

## Health monitoring

- **Probes:** liveness is `/api/v1/health/live` (process only); readiness is `/api/v1/health/ready` (PostgreSQL reachable within `READINESS_TIMEOUT`). The API image's health check runs `tableflow-api -healthcheck`.
- **Alerts:** readiness failing for longer than 1 minute; any `level=ERROR` log line; `db pool` logs with rising `waited_acquires` or `avg_acquire_ms` above about 50 ms (pool saturation); 5xx rate above 0.5% at the proxy; certificate expiry within 14 days.
- **Logs:** JSON on stdout with request IDs. Tokens, passwords and API keys are never logged.

## Backups and restore

- **Backups:** nightly `pg_dump -Fc` of the database (plus continuous WAL archiving if the platform offers point-in-time recovery). Store backups encrypted and access-controlled, outside the database host. Retention is an operator decision; 30 days is proposed.
- **Restore procedure:** create an empty database with the same roles, then `pg_restore --no-owner --role=tableflow_owner --exit-on-error`. Confirm that `goose_db_version` matches the release, then point the API at it.
- **Drill:** `tooling/runtime/restore-drill.sh` dumps and restores locally and reconciles settlements, snapshot totals, refunds, claims and the loyalty ledger (profiles equal ledger sums). It also checks the app role and migration version. Run it before launch and after every schema-changing release.

## Migrations and recovery

- **Before each release:** take a backup, then run `migrate up`. Migrations are transactional per file; a failure rolls that file back and stops.
- **After a failure:** fix forward with a new migration. `migrate down` is only for rehearsals on data that does not exist yet (Down scripts can lose data once real bills exist).
- **Compatibility:** migrations must be backward compatible with the running API version (add, backfill, switch, then remove in a later release). The fresh and upgrade paths run in `make verify` (`TestFreshMigrationAndDatabaseSmoke`, `TestUpgradeFromPreviousRelease`).
- **Rollback:** redeploy the previous images. The previous API version runs against the new schema because migrations are additive.

## Email (Resend, ADR-0005)

- **Configuration:** `SMTP_ADDR=smtp.resend.com:587`, `SMTP_TLS=starttls`, `SMTP_USERNAME=resend`, and `SMTP_PASSWORD=<API key>` from the secret store.
- **Sender:** `MAIL_FROM` must use the verified domain.
- **Without a domain (test mode):** `MAIL_FROM=TableFlow <onboarding@resend.dev>` works, but Resend delivers only to the Resend account owner's address. Other recipients fail with a 550, logged as `mail send failed`. That is fine for staging and not usable for guests.
- **Before launch:**
  1. Verify the domain in Resend.
  2. Disable open and click tracking for these messages.
  3. Send one member verification, one password reset and one staff activation to authorised test recipients.
  4. Confirm that the links work and that nothing sensitive appears in logs.

## Open decisions (block production, not staging)

- Hosting provider and region, and managed PostgreSQL versus a self-run container.
- Public hostnames and DNS for the admin and PWA.
- Backup storage, retention and who can restore.
- Resend sending domain and verification (the API key works; delivery verified in test mode).
- Log retention and alerting destination.
- Data retention and deletion procedures (security spec) for members, guest sessions and audit.
