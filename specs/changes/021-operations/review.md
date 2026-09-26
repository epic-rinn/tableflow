# Review: 021-operations

Date: 2026-09-27 (M6 milestone gate, ADR-0004). Reviewer: Claude — **self-review**. Scope: images, staging stack, Caddy HTTPS, API health probe, restore drill, deployed-origin checks, runbook. Result: **pass for the delivered scope; task blocked on live email**.

## Findings

| Severity | Location | Evidence | Impact | Resolution |
| --- | --- | --- | --- | --- |
| P2 | `deploy/staging/Caddyfile` | First HTTPS probe: no `Strict-Transport-Security` | Browsers could be downgraded before HSTS is learned | HSTS added; the deployed-origin check asserts it |
| P2 | `staging.env.example` | Set `SMTP_USERNAME=resend` without a password; the API refused to start | Broken first deployment | Both empty until the key exists; the runbook explains |
| P3 | Rehearsal TLS | Chrome refuses service workers under a local-CA certificate error | Rehearsal-only | `STAGING_INSECURE_TLS=1` launches Chrome without verification; real certificates need no override |
| — | Live email (ADR-0005) | Not possible without the user's verified domain and API key | Member verification, reset and staff activation emails cannot be delivered | **Blocked**; next action is the user providing the domain and key via a secret store |

**Checked:**
- **Restore drill:** reconciles settlements, snapshot totals, refunds, claims, ledger and profiles, the app role and the migration version on the E2E and 1M-line databases.
- **Migration paths:** fresh and upgrade migrations run in `make verify`.
- **Images:** exclude docs, specs and AI files.
- **Deployed origins:** host-only Secure cookies, the origin guard, private no-store responses, and service-worker isolation all hold.
- **Secrets:** `staging.env` is git-ignored and was never staged.

## Verification

| Command / test | Result |
| --- | --- |
| `make verify` (final, 2026-09-26T22:58Z UTC; macOS arm64, Go 1.27.1, Node 24.21.0, pnpm 12.6.0, postgres:18.6, Chrome) | **passed**: 162 Go tests (race, PostgreSQL, none skipped), admin 16/16, PWA 25/25, cross-app journey 1/1, artifact check (2,881 files / 145 sentinels, no leaks) |
| Rehearsal | [evidence/staging-rehearsal.md](evidence/staging-rehearsal.md), [restore-drill.json](evidence/restore-drill.json), [restore-drill-perf.json](evidence/restore-drill-perf.json), [image-inspection.json](evidence/image-inspection.json) |

## Delivery decision

No open P0/P1 in the delivered scope. Status: **blocked** (live email only); everything else is verified.
