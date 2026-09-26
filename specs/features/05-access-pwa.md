# Identity, staff operations, and PWA

Status: specified; staff identity (ACC-002 staff part, ACC-003 for staff routes, ACC-004 staff sessions) implemented in MVP-02, guest capabilities/sessions (ACC-001) in MVP-03 and member identity (ACC-002 member part) in MVP-04; guest screens (queue, dining, orders, bill, assistance, account) are redesigned in UI-02; PWA-001–003 (manifest, install guidance, service worker limited to public static assets, offline page, safe update) were implemented in MVP-17; the PWA-004/005 audit is MVP-18.

Visual design and mobile patterns: [UI design](../product/ui-design.md) (UI-003).

Guest/member screens and PWA behavior belong to `src/pwa`. Staff operations and login belong to the separate [admin panel](06-admin.md) in `src/admin`. Both use `src/api`; keep customer and staff sessions isolated by origin.

## Access and operations

- **ACC-001** Guest queue and visit access use cryptographically random capabilities, distinct from display IDs. Exchange a QR fragment token through a POST body into an HttpOnly, Secure, SameSite cookie session; remove the fragment from visible history. Store token hashes, allow revocation/rotation, and never log raw credentials. Multiple diners can exchange the active dining token; it is not consumed by the first scan.
- **ACC-002** Member signup/login uses email and password for MVP, with maintained password hashing and email verification/reset tokens before live use. Email transport remains an adapter: local development uses Mailpit, and production uses Resend SMTP per [ADR-0005](../decisions/0005-production-email.md). Domain, credentials and live delivery verification remain deployment prerequisites. Staff accounts are manager-provisioned and cannot self-register as staff. Staff role changes revoke affected sessions.
- **ACC-003** Go enforces branch/resource authorization on every operation. Roles: host/server (queue, seats, orders, assistance), kitchen (line preparation state), cashier (bills, settlement), manager (configuration, overrides, refunds, reports). A staff user may have multiple roles. No UI-only protection.
- **ACC-004** Cookies need Origin/CSRF protection for mutations, session expiry/revocation, and rate-limited authentication/capability exchange. Do not put member or staff tokens in localStorage. Responses with session/queue/visit/member data are private and `no-store`.
- **OPS-001** Manager configures tables, menu, business/loyalty policy, and staff accounts. Audit entries identify actor, action, resource, timestamp, request ID, and reason for overrides/financial actions, without passwords or raw tokens.
- **OPS-002** Daily summary includes ticket counts/no-shows, visits, sales/refunds by method, and loyalty earned/reversed. Filter by branch/business date, paginate detail, and cap the interactive date range at 31 days.

## PWA behavior

- **PWA-001** Provide a manifest, icons, HTTPS deployment, service-worker update handling, and install guidance only where supported. Normal browser use remains fully supported; camera permission is optional because the device camera can open the QR link.
- **PWA-002** Cache only versioned public static assets and a generic offline page. Exclude `/api/`, QR entry routes, personalized documents, auth pages, and Next.js RSC/data responses from service-worker caches. Never cache a bill or a member response, even if its URL resembles a public page.
- **PWA-003** Offline/stale screens disable mutations and show the last successful refresh time. Do not queue orders, settlement, or loyalty actions for background replay. A timed-out submit retains its idempotency key for explicit retry/reconciliation.
- **PWA-004** Thai/English labels, readable currency, keyboard access, field errors, accessible status announcements, and staff-assisted alternatives support the complete core journey. Do not promise push notifications while the browser is closed.
- **PWA-005** Poll according to [system architecture](../architecture/system.md); pause hidden tabs, cancel on navigation, back off on failures, and fetch on reconnect. Invalidate stale UI after mutations and do not reset another device's cart.

## Acceptance scenarios

| ID | Given / When / Then |
| --- | --- |
| ACC-A1 | Given a member or staff session for another branch/resource, then read and mutation requests fail without exposing resource existence (ACC-003). |
| ACC-A2 | Given a rotated dining token, then old links and derived guest sessions fail; the new token can serve multiple authorized diners (ACC-001). |
| ACC-A3 | Given forged Origin/CSRF or revoked staff access, then mutations fail (ACC-002/004). |
| PWA-A1 | Given an offline phone after viewing a bill, then the offline page never restores a cached private bill and order/payment actions cannot submit (PWA-002/003). |
| PWA-A2 | Given a hidden tab, then recurring polls stop; focus or reconnect fetches fresh state with visible errors if unavailable (PWA-005). |
| PWA-A3 | Given a service-worker upgrade, then active carts are not silently discarded and old private resources are never cached (PWA-001/002). |
