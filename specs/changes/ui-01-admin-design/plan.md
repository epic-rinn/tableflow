# Change: ui-01-admin-design — Admin design system and redesign

Status: implemented-unverified (MU gate with UI-02 pending; `make verify` already passes). Date: 2026-09-27. Scope owner: Claude. Task: UI-01. Verification: MU gate (ADR-0004), together with UI-02.

## Problem and behavior

Maps UI-001, UI-002, UI-004 and UI-005, plus the presentation of ADM-002–006 ([UI design](../../product/ui-design.md), [ADR-0006](../../decisions/0006-ui-stack.md)). The admin screens work but are unstyled.

This change introduces Tailwind CSS v4 and shadcn/ui, a sidebar app shell, and a redesign of every existing workspace to the modern admin-panel language. There are **no API, authorization or behaviour changes**.

## Decisions (reversible)

- **Setup:** shadcn/ui via its CLI (`components.json`, style "new-york", neutral base re-tinted with TableFlow tokens), with components generated into `components/ui/`. Versions are pinned exactly in `package.json`.
- **Fonts:** Inter Variable and Noto Sans Thai Variable from `@fontsource-variable/*`, imported in the root layout. There are no `next/font/google` or runtime third-party requests.
- **Theme:** `next-themes` with the `class` strategy; light by default and dark via a toggle.
- **Messages:** inline `Alert`s keep `role="alert"`/`"status"` inside `main`, because the tests rely on them. No toast library in UI-01.
- **Test stability:** accessible names in tests stay stable. Where a button becomes an icon button or moves into a menu or dialog, the test changes in the same commit and this plan records why.
- **Accessibility scan:** `@axe-core/playwright` in a new `a11y.spec.ts`, scanning login, host, kitchen, cashier, receipts, menu, configuration, staff and charges. Serious or critical violations fail.

## Verification map

| Requirement | Test / evidence | Status |
| --- | --- | --- |
| UI-A1 journeys and axe | existing admin specs + `a11y.spec.ts` in `make verify` | passing (admin 15/15; axe clean at WCAG 2.2 AA tags, light and dark) |
| UI-A3 desktop/tablet shell, keyboard dialogs | screenshots of 9 screens at 1280 and 1024 in `tmp/playwright/admin-screens`; Enter/Escape/focus-return check in `a11y.spec.ts` | passing |
| Client-JS budget (≤400 KiB gzip first load) | [before](evidence/client-js-before.json) 170–178 KiB → [after](evidence/client-js-after.json) 203–234 KiB gzip; CSS 0.7 → 16 KiB | within budget |

## Implementation notes

- **Tooling and dependencies:**
  - The shadcn CLI shelled out to a global pnpm 10 that cannot read the pnpm 12 lockfile, and it pointed the generated components at an unrelated npm package, `cn`. That package was removed, imports now use `@/lib/utils`, and dependencies were installed with the pinned pnpm.
  - `cn@0.2.6` remains only as a transitive dependency of the `shadcn` CLI (a dev dependency, not shipped).
  - The generated `use-mobile` hook was rewritten with `useSyncExternalStore` to satisfy the React lint rules.
- **Test changes (UI-002):**
  - close-empty, new QR, reject and cancel line moved into reason dialogs, so `host.spec.ts` opens the dialog;
  - payment and refund gained a final confirmation dialog, so `cashier.spec.ts` confirms it;
  - accessible names otherwise unchanged.
- **Native elements kept:** native `<select>` (styled) and native checkboxes in forms submitted via `FormData`, because tests drive them with `selectOption`/`check` and they behave best on tablets.
- **Contrast:** the first axe run failed on colour contrast (semantic badge text, destructive buttons in both themes, the login brand mark). Tokens were tuned with a computed contrast check to at least 4.5:1. Destructive buttons use a separate solid token.
