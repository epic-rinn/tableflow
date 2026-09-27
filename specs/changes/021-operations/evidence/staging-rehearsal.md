# Staging rehearsal results (2026-09-27, local)

Stack: `deploy/staging/compose.yaml` (PostgreSQL 18.6, migrate job, API, admin, PWA, Caddy 2.10 with `CADDY_TLS=internal`), hosts `admin.tableflow.localhost` / `pwa.tableflow.localhost`. Secrets generated into the git-ignored `staging.env`.

## Deployment

- **Migrations:** the migrate job applied all 12 migrations as `tableflow_owner`. The API started only after it completed and connected as `tableflow_app`.
- **Health:** the API health check `tableflow-api -healthcheck` reported healthy. `/api/v1/health/ready` returned 200 through both HTTPS origins.
- **Email configuration (ADR-0005):**
  - with `SMTP_USERNAME` set and no password, the API refused to start ("set both SMTP_USERNAME and SMTP_PASSWORD, or neither");
  - with neither set, it started and logged `mail delivery not configured … every send will fail`.
- **Bootstrap:** `tableflowctl bootstrap-branch` in the API image created the branch and manager and printed a one-time activation token (kept out of logs and evidence). A second bootstrap is refused ("a branch already exists").

## Deployed-origin checks (`src/pwa/deploy-checks/origins.spec.ts`)

| Check | Result |
| --- | --- |
| HTTPS on both origins; HSTS, nosniff, `no-referrer`, no `x-powered-by`; readiness 200 | passed (HSTS was added to the Caddyfile after the first probe showed it missing) |
| Staff activation and sign-in; `__Host-tf_staff` Secure, HttpOnly, SameSite=Strict, host-only; not sent to the PWA host | passed |
| `/api/v1/sessions/current` is `private, no-store` | passed |
| Forged `Origin` with the staff cookie gets 403 | passed |
| No service worker on the admin origin | passed |
| PWA service worker controls pages over HTTPS; caches hold only static assets, icons, offline page and manifest | passed |
| `__Host-tf_anon` Secure, HttpOnly, SameSite=Lax, host-only; no cookies on the admin host | passed |

The staff test runs once per activation token (one-time by design). It passed on the first run and was skipped on the re-run.

**Local CA:** Chrome refuses service-worker scripts under a certificate error even when the page ignores it ("An SSL certificate error occurred when fetching the script"). The local rehearsal therefore starts Chrome with `--ignore-certificate-errors` (`STAGING_INSECURE_TLS=1`). With publicly trusted certificates no override is used.

## Artifact inspection

Filesystem listings of all three images contain no specs, docs, tooling, AI instruction files, OpenAPI documents or Word files ([image-inspection.json](image-inspection.json)).

| Image | Files | Size |
| --- | --- | --- |
| API | 1,463 | 50 MB |
| Admin | 9,015 | 407 MB |
| PWA | 8,988 | 406 MB |

Both Next.js images are mostly the Node base plus a 43 MB standalone output.

## Environment note

The first image build failed with "no space left on device" in the Docker VM. My `qualify.sh` runs had left five anonymous PostgreSQL volumes (about 6.3 GB). I removed those after matching their creation times to my runs, and fixed the script to use `docker rm -fv`. Other projects' images and volumes were left untouched.

## Live email via Resend test mode (2026-09-27)

The user supplied an existing Resend API key in the git-ignored `staging.env`; it never passed through chat, files under version control, or logs. There is no verified domain: sender `onboarding@resend.dev` (Resend test mode).

| Step | Result |
| --- | --- |
| API start with `SMTP_USERNAME=resend` + key | No "delivery not configured" error; configuration accepted |
| Signup to an address that is not the account owner | HTTP 202 (generic response). Resend completed STARTTLS and authentication, then refused the recipient: `550 You can only send testing emails to your own email address`. Logged as `WARN mail send failed`, no token in the log |
| Signup (verification) and password-reset request to the Resend account's own address | HTTP 202 / 202; Resend accepted both messages (0 send failures) |
| Logs scanned for API keys or 43-character capability tokens | 0 matches |
| Inbox receipt | **Confirmed by the user**: both the verification and password-reset emails arrived |

**Limits:**
- Test mode delivers only to the Resend account owner's address, so production still needs a verified sending domain (MVP-22 checklist item 10).
- Staff activation is not emailed in this system; managers copy the one-time link from the Staff page.
- Disabling open and click tracking is a Resend dashboard setting for the user to confirm once a domain exists.
