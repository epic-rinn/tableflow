# Review: ui-01-admin-design

Date: 2026-09-27 (MU milestone gate, ADR-0004). Reviewer: Claude — **self-review** (code, accessibility and visual review of screenshots). Scope: Tailwind/shadcn setup, app shell, all admin workspaces, test changes. Result: **pass**.

## Findings

| Severity | Location | Evidence | Impact | Resolution |
| --- | --- | --- | --- | --- |
| P1 | `src/admin/package.json` (shadcn CLI) | The CLI added an unrelated npm package `cn` and imported `cn` from it in every component | An unreviewed third-party runtime dependency (supply-chain risk) shipped to staff browsers | Fixed: package removed, imports use `@/lib/utils`. `cn@0.2.6` remains only under the dev-only `shadcn` CLI |
| P2 | Tokens (`app/globals.css`), destructive button | First axe run: colour-contrast (serious) on badges, destructive buttons (light and dark) and the login brand mark | Text below 4.5:1 for staff | Fixed: tokens tuned with a computed contrast check; separate `destructive-solid` token; axe clean in both themes |
| P2 | `app/(staff)/page.tsx` | Importing a value from a `"use client"` module into a server component | Would yield a client reference instead of the icon map | Fixed before the first run: icons moved to `lib/workspaceIcons.ts` |
| P3 | `components/MenuEditor.tsx` `Field` | Ids generated with `Math.random()` during render | Hydration mismatch warnings | Fixed: `useId` |
| P3 | Layout (screenshots) | Cashier table buttons were cramped; Thai names truncated in kitchen availability | Readability; the spec forbids truncating Thai | Fixed: wrapping layout, no truncation |

**Checked:**
- There are no API or behaviour changes; every mutation still goes through the same idempotent calls.
- Payment and refund now need an extra confirmation, and close-empty, new QR, reject and cancel line need a reason dialog.
- The keyboard dialog path works: Enter opens, focus moves to the reason field, and Escape closes and returns focus.
- Theme toggle works, the sidebar exposes `nav[aria-label=Workspaces]` with `aria-current`, and there are no third-party font or CDN hosts.

**Tests changed, with reasons:**
- `host.spec.ts`: close-empty now goes through a dialog.
- `cashier.spec.ts`: payment and refund confirmation dialogs.
- `staff.spec.ts`: the copy button was named so it didn't collide with the "Activation link" label.

## Verification

| Command / test | Result |
| --- | --- |
| `make verify` (final, 2026-09-27; macOS arm64, Go 1.27.1, Node 24.21.0, pnpm 12.6.0, postgres:18.6, Chrome) | **passed**: Go suite unchanged and green, admin 15/15 browser tests (incl. 4 axe/screenshot/keyboard tests), PWA 17/17 at 390×844 (incl. 2 axe/screenshot/third-party tests), artifact check |
| Earlier MU runs | Run 1: staff spec strict-mode clash on "Activation link" (the copy button name). Run 2: axe contrast failures on 3 screens. Run 3: login brand mark contrast. All fixed |
| Client JS | [before](evidence/client-js-before.json) 170–178 → [after](evidence/client-js-after.json) 203–234 KiB gzip (budget 400); CSS 16 KiB |

## Delivery decision

No open P0/P1. Status: **done** (MU gate).
