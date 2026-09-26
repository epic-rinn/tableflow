# UI design

Status: accepted direction (2026-09-27); implemented in UI-01 (admin) and UI-02 (PWA), MU gate passed 2026-09-27. Stack: [ADR-0006](../decisions/0006-ui-stack.md). This spec describes presentation only: behaviour, authorization and data rules stay in the feature specs and in Go.

## Principles

- **Two products, one family.**
  - The admin is a dense, calm desktop/tablet workspace for staff under time pressure.
  - The PWA is a friendly, thumb-first mobile experience for guests.
  - The two apps share type, spacing scale and semantic colours, but not layouts.
- **State is always visible.** Every operational object shows its state as a coloured badge with text; colour is never the only signal. The states are table, ticket, order line, visit, bill and assistance. Stale data shows its last-refresh time.
- **Server acknowledgement only.** Success UI appears after the API confirms. While waiting, controls show a pending state; ambiguous failures say so and offer a safe retry (the same idempotency key).
- **Bilingual and local.**
  - Thai and English labels; THB with `฿` and two decimals.
  - Dates in Asia/Bangkok.
  - Thai text never truncated mid-cluster.
- **Accessible by construction.**
  - WCAG 2.2 AA contrast.
  - Visible focus.
  - Touch targets of at least 44×44 px on the PWA and 36 px on the admin.
  - `prefers-reduced-motion` respected.
  - Every control keeps a stable accessible name.

## Tokens

The palette is TableFlow's own: primary green-teal, neutral slate and warm accent. Grab's brand colours, logo, name and illustrations are not reused. Semantic tokens:

| Token | Meaning |
| --- | --- |
| `primary` | Main action |
| `success` | Served, paid, available |
| `warning` | Called or held, preparing, settling |
| `destructive` | Rejected, cancelled, no-show, refunds |
| `info` | Submitted, acknowledged |
| `muted` | Inactive, historical |

Radius:
- 8 px on the admin;
- 12–16 px on PWA cards and sheets;
- pill-shaped for PWA chips and badges.

Spacing: a 4 px scale. Shadows are subtle, used for elevation (sheets, sticky bars) only.

## Admin (`src/admin`) — modern admin panel

- **App shell:**
  - collapsible left **sidebar**, with workspaces filtered by role, icons and labels;
  - a top bar with branch name, staff name/roles menu (sign out), a theme toggle and a live freshness indicator;
  - on tablets the sidebar becomes a sheet.
- **Pages:** a page header (title, description, primary action) and content in cards.
- **Host board:**
  - a table grid as cards coloured by state, showing capacity, needs and claim;
  - the queue as a list with position, party size, needs and wait time;
  - actions in card footers or dropdown menus;
  - dialogs for seat/move/close with reason fields.
- **Kitchen:** a large-type ticket board, one column per state, one card per line with elapsed time. Buttons advance the state; a sold-out panel uses switches.
- **Cashier:**
  - a split view: an occupied-table picker and QR paste on the left, the itemised bill with a totals summary on the right;
  - payment confirmation in a dialog that repeats the exact amount, method and verification note;
  - reopen needs a reason.
- **Receipts:** a data table with search and pagination, and a receipt detail view.
- **Refunds:** recorded from the receipt detail in a destructive-styled confirmation dialog.
- **Menu, configuration, staff and charges:** form layouts with inline field errors, tabs where helpful, and unsaved-change warnings.
- **Feedback:**
  - inline `Alert`s (role alert/status) for results that tests and screen readers rely on;
  - toasts only as a supplement;
  - skeletons while loading;
  - empty states with guidance.

## PWA (`src/pwa`) — Grab-style mobile experience

Patterns follow modern super-app ordering apps, without copying any brand.

- **Layout:**
  - mobile-first at 360–430 px, centred with a maximum width of about 480 px on larger screens;
  - safe-area insets;
  - no horizontal scroll except category chips.
- **Header:** a coloured hero header with the restaurant name, table label or queue number, and a status pill.
- **Queue tracking:** a big ticket-number card with position, a status stepper (waiting → called → seated) and a prominent "table ready" state with the table label. Cancel is behind a confirmation sheet.
- **Menu:**
  - sticky, horizontally scrollable category chips with scroll-spy;
  - item rows or cards with a thumbnail placeholder, Thai name first and English second, price, a sold-out badge and a round "+" add button;
  - item options open in a **bottom sheet**: required/optional groups with min/max hints, a quantity stepper, a note, and "Add ฿x" as the primary button.
- **Cart:**
  - a **sticky bottom bar** ("View cart · 3 items · ฿240"), per phone;
  - the cart sheet has editable lines and "Send order";
  - a MENU_CHANGED conflict highlights the affected lines.
- **Orders:** a timeline of confirmed orders with line status chips (Sent, Accepted, Preparing, Ready, Served, Rejected with reason).
- **Bill:** a receipt-style card with lines, subtotal, service charge, tax and total, plus a "pay at the counter" note. There is never a pay button.
- **Help:** a "Need something?" card with quick-action chips (call staff, allergy question, ask for the bill) and each request's live status, always visible on the dining page. It is a card rather than a sheet so request status stays on screen.
- **Account and member:** a bottom tab bar (Home/Menu, Orders, Account) once loyalty screens exist (M4), with clean auth forms.
- **Offline and stale states:** a top banner with the last refresh time; mutations disabled.

## Requirements

- **UI-001** Both apps use the shared token set and shadcn/ui primitives from ADR-0006. There is no ad-hoc per-page styling beyond Tailwind utilities.
- **UI-002** The admin uses the sidebar app shell, state badges, dialogs for consequential actions (payment, refund, override, close, rotate QR) and data tables for lists over 20 rows.
- **UI-003** The PWA implements the mobile patterns above: hero header, category chips, option bottom sheet, sticky cart bar, order timeline, receipt-style bill and help card.
- **UI-004** Accessibility and locale requirements (PWA-004, ADM-006) hold after the redesign. Accessible names and roles used by the browser tests stay stable, or the tests change in the same commit with a stated reason.
- **UI-005** No third-party runtime asset hosts (fonts, icons, images). There are no brand assets from other companies, and item images are placeholders until menu images are specified.

## Acceptance

| ID | Scenario |
| --- | --- |
| UI-A1 | Given the redesigned apps, when `make verify` runs, then all existing browser journeys pass, and an automated accessibility scan (axe) of each main screen reports no serious or critical violations (UI-001/004). |
| UI-A2 | Given a 390×844 phone viewport, then join, queue, menu → options sheet → cart → send, orders, bill and help are usable one-handed without horizontal scroll; screenshots are saved as review evidence (UI-003). |
| UI-A3 | Given a 1280×800 and a 1024×768 viewport, then every admin workspace renders in the app shell, with dialogs and tables keyboard-operable (UI-002). |
| UI-A4 | Given the production build, then PWA first-load JavaScript for `/t` stays within the [performance](../quality/performance.md) client budget, and no request leaves for a third-party host (UI-005). |
