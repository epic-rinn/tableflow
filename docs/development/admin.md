# Admin panel development

Runtime location: `src/admin/`. App not initialized. This is a separate Next.js App Router/TypeScript application for restaurant staff and managers, sharing the Go API with the customer PWA.

Provide staff login and role-specific workspaces: host queue/table board, server orders/assistance, kitchen preparation board, cashier settlement/receipts, manager menu/configuration/staff/reports/audit. Hiding a screen is not authorization; Go checks every action and resource.

M0 creates this project's own package manifest, pinned tooling, Next.js configuration, and tests. Use `app/`, `features/`, `components/`, and `lib/api/` relative to this runtime project. The admin is a browser application in MVP; customer PWA installation and service-worker behavior do not apply to it.

Use the admin origin's `/api/v1` proxy to the shared Go service, host-only staff cookies, no private shared caching, and server-only `API_INTERNAL_URL` for server reads. Stop polling on hidden tabs and label stale operational data. Disable unsafe actions during outages and preserve explicit confirmation for payment/refund operations.

Read [admin requirements](../../specs/features/06-admin.md), [security](../../specs/architecture/security.md), and [performance](../../specs/quality/performance.md). Document actual setup/check commands here once M0 implements them. Keep docs and AI instructions outside `src/admin/`.
