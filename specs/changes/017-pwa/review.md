# Review: 017-pwa

Date: 2026-09-27 (M5 milestone gate, ADR-0004). Reviewer: Claude — **self-review**. Scope: manifest, icons, offline page, `public/sw.js`, registration and update banner, install hint, smoke-test change. Result: **pass**.

## Findings

| Severity | Location | Evidence | Impact | Resolution |
| --- | --- | --- | --- | --- |
| P3 | `InstallHint.tsx` | `beforeinstallprompt` and iOS detection are browser-dependent and not browser-tested | Wrong or missing guidance on some browsers; the site itself is unaffected | Accepted: guidance is optional and dismissible (PWA-001) |
| P3 | `public/offline.html` | Not included in the axe scan | Possible contrast or structure issues on the offline page | Colours reuse the checked tokens; add it to the scan in MVP-19 |

**Checked (in `sw.js` and by tests):**
- **Never intercepted:** non-GET requests (no background replay), other origins, `/api/*`, and RSC or data requests (header or `_rsc`).
- **Pages:** network-only, with the generic offline page on failure.
- **Caches:** after visiting dining, bill and account pages they hold only `/_next/static`, icons, the offline page and the manifest (PWA-A1). Offline `/t`, `/account` and `/q` show the generic page, never the bill or menu.
- **Updates:** a new version waits for the guest, and the cart survives the reload (PWA-A3).
- **Worker script:** served with `no-cache`.
- **Admin:** has no worker, and the admin smoke test asserts it (ADM-A2). The worker is scoped to the PWA origin only.

## Verification

| Command / test | Result |
| --- | --- |
| `make verify` (2026-09-27; macOS arm64, Go 1.27.1, Node 24.21.0, pnpm 12.6.0, postgres:18.6, Chrome) | **passed**: Go suite green (race, PostgreSQL, none skipped), admin 16/16, PWA 25/25, artifact check |
| MVP-17 tests | PWA `pwa.spec.ts` (installable; caches and offline; update keeps cart); admin `smoke.spec.ts` (no worker) |

## Delivery decision

No open P0/P1. Status: **done** (M5 gate).
