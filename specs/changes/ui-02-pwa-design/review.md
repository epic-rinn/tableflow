# Review: ui-02-pwa-design

Date: 2026-09-27 (MU milestone gate, ADR-0004). Reviewer: Claude — **self-review** (code, accessibility and visual review of 390×844 screenshots). Scope: Tailwind/shadcn setup, mobile shell, join, QR entry, queue tracking, dining (menu, options sheet, cart sheet, orders, bill, help), account pages. Result: **pass**.

## Findings

| Severity | Location | Evidence | Impact | Resolution |
| --- | --- | --- | --- | --- |
| P1 | `src/pwa/package.json` (shadcn CLI) | The same stray `cn` package as in UI-01 | Unreviewed runtime dependency on guest phones | Fixed: removed; imports use `@/lib/utils` |
| P3 | `tests/queue.spec.ts` | Located text through the CSS class `.position` | Breaks the UI-004 rule (tests use roles and text) | Fixed: the test scopes to the ticket region by accessible name |
| P3 | `tests/a11y.spec.ts` cart sheet | axe reported contrast on dark text inside the sheet | False positive while the sheet was still fading in | Scan now waits for animations; clean with unchanged colours |
| P3 (gap) | `OptionSheet` | The E2E menu has no option groups, so the bottom sheet is not exercised in a browser | Regressions in option selection would only be caught by typecheck and review | Add an option item to the E2E fixture in MVP-19 |

**Checked:**
- Carts stay per phone (sessionStorage) and survive reload.
- The idempotent send logic is unchanged.
- MENU_CHANGED highlights cart lines and the cart bar turns amber.
- Ordering is hidden when the visit is not open.
- The bill is read-only with no pay action.
- Allergy messaging is kept.
- Touch targets are at least 44 px, content is capped at `max-w-md` with safe-area padding, and only same-origin requests are made (test).
- No Grab name, logo, colours or illustrations are used.

**Tests changed, with reasons:**
- `dining.spec.ts`: the cart sits behind the sticky bar and sheet (UI-003).
- `queue.spec.ts`: locator change (UI-004).
- All PWA tests run at 390×844 with touch.

## Verification

| Command / test | Result |
| --- | --- |
| `make verify` (final, 2026-09-27; macOS arm64, Go 1.27.1, Node 24.21.0, pnpm 12.6.0, postgres:18.6, Chrome) | **passed**: Go suite unchanged and green, admin 15/15 browser tests (incl. 4 axe/screenshot/keyboard tests), PWA 17/17 at 390×844 (incl. 2 axe/screenshot/third-party tests), artifact check |
| Earlier MU run | Cart-sheet axe false positive during animation; test waits for animations; re-run passed |
| Client JS | [before](evidence/client-js-before.json) `/t` 174.4 → [after](evidence/client-js-after.json) 205.5 KiB gzip (budget 250); CSS 10 KiB; fonts per page about 74–104 KiB (Inter Latin 47 KiB + Noto Thai subset 26 KiB, Noto Latin only if needed) |

## Delivery decision

No open P0/P1. Status: **done** (MU gate).
