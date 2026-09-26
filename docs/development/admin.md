# Admin panel development

Runtime location: `src/admin/`. Staff workspaces implemented through M3 (functional styling); the modern admin-panel redesign is UI-01. This is a separate Next.js App Router/TypeScript application for restaurant staff and managers, sharing the Go API with the customer PWA.

Provide staff login and role-specific workspaces: host queue/table board, server orders/assistance, kitchen preparation board, cashier settlement/receipts, manager menu/configuration/staff/reports/audit. Hiding a screen is not authorization; Go checks every action and resource.

The project has its own `package.json`, `pnpm-lock.yaml`, Next.js configuration, and Playwright smoke tests (dev port 3001). Use `app/`, `features/`, `components/`, and `lib/api/` relative to this runtime project. The admin is a browser application in MVP; customer PWA installation and service-worker behavior do not apply to it.

Use the admin origin's `/api/v1` proxy to the shared Go service, host-only staff cookies, no private shared caching, and server-only `API_INTERNAL_URL` for server reads. Stop polling on hidden tabs and label stale operational data. Disable unsafe actions during outages and preserve explicit confirmation for payment/refund operations.

Read [admin requirements](../../specs/features/06-admin.md), [security](../../specs/architecture/security.md), and [performance](../../specs/quality/performance.md). Screens: `/login`, `/activate` (token read from the URL fragment, then removed from history), role-filtered navigation, `/staff` (manager: invite, roles, activation links, deactivation) and host workspace `/host` (queue board, assisted join, call/no-show/seat, move/depart/close/ready, new dining QR; 3 s polling with hidden-tab pause and backoff), manager `/configuration` (tables and seating groups), `/kitchen` (line workflow and sold-out toggles), `/menu` (manager menu editor), `/visits/<id>` (orders, assisted ordering, line cancellation), an assistance board on `/host`, cashier `/cashier` (bill by table or pasted dining QR, begin/reopen/confirm settlement), `/receipts` and `/receipts/<id>` (history, manager full refund), and manager `/charges` (versioned charge/tax policy). `NEXT_PUBLIC_PWA_URL` builds the tracking/dining links shown to staff. The `(staff)` layout reads the session server-side via `lib/api/server.ts`; browser calls use `lib/api/client.ts`. Setup and checks: [setup](setup.md) (`make admin-check`, `make smoke`; the browser tests need `make verify`). `proxy.ts` forwards `/api/v1/*` to `API_INTERNAL_URL` at request time. Keep docs and AI instructions outside `src/admin/`.

## UI conventions (from UI-01)

Stack and visual language: [ADR-0006](../../specs/decisions/0006-ui-stack.md) and [UI design](../../specs/product/ui-design.md).

- **Components:**
  - Tailwind CSS v4 utilities, with shadcn/ui components in `components/ui/` (generated source, reviewed like any code) and `cn()` in `lib/utils.ts`;
  - icons from `lucide-react`;
  - add a component with the shadcn CLI, pin the resulting dependencies, and never edit `node_modules`.
- **Layout:** the sidebar app shell lives in the `(staff)` layout. Pages use a page header plus cards; tables longer than 20 rows use the data-table pattern with keyset pagination from the API.
- **Actions and messages:**
  - payment, refund, overrides, close-empty and new QR go through a confirmation `Dialog`;
  - results use inline `Alert`s with `role="alert"`/`"status"`, and toasts are supplementary only.
- **State display:** use the shared `StateBadge` mapping (text plus colour). Show freshness in the top bar and on polling panels.
- **Tests:** keep accessible names stable; tests locate by role, label and name, never by class.
- **Themes:** light and dark via the `class` strategy. Tokens live in `app/globals.css`; do not hard-code colours in components.

