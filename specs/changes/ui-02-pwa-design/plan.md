# Change: ui-02-pwa-design — PWA design system and redesign

Status: done (MU gate passed 2026-09-27). Date: 2026-09-27. Scope owner: Claude. Task: UI-02. Verification: MU gate (ADR-0004), together with UI-01.

## Problem and behavior

Maps UI-001, UI-003, UI-004 and UI-005, plus the presentation of PWA-003–005 ([UI design](../../product/ui-design.md)). The guest screens work but are unstyled.

This change applies the Grab-style mobile patterns with TableFlow's own tokens (the same values as the admin):
- hero header, queue ticket card with stepper;
- category chips, item rows with round add buttons, option bottom sheet;
- sticky cart bar and cart sheet;
- order timeline, receipt-style bill, help card (kept on-page so request status stays visible);
- restyled join, QR entry and account pages.

**No API, cart, idempotency or behaviour changes.** No service worker (MVP-17). No brand assets from Grab.

## Decisions (reversible)

- **Stack:** the same stack as UI-01 (Tailwind v4, shadcn/ui with Radix, lucide icons, self-hosted Inter and Noto Sans Thai), light theme only.
- **Layout:** mobile-first with content capped at `max-w-md`, safe-area insets, and touch targets of at least 44 px.
- **Sheets:** option selection and the cart use shadcn `Sheet` from the bottom. Accessible names used by the tests stay stable, or the tests change in the same commit with a reason.
- **Test viewport:** PWA browser tests run at 390×844 (Playwright device profile Pixel 7 class or an explicit viewport), and `a11y.spec.ts` scans the main screens.

## Verification map

| Requirement | Test / evidence | Status |
| --- | --- | --- |
| UI-A1 journeys and axe | PWA specs + `a11y.spec.ts` | passed (PWA 17/17) |
| UI-A2 phone journey, screenshots | all PWA journeys at 390×844; screenshots of home, join, account, dining, cart sheet, bill | passed |
| UI-A4 ≤250 KiB gzip first-load JS on `/t`, no third-party requests | 174.4 → 205.5 KiB; request-host check passes | passed |
