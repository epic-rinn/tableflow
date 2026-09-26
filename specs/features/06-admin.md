# Restaurant admin panel

Status: specified; ADM-001 implemented in MVP-02, ADM-002/003 in M1–M2, ADM-004 in M3. ADM-005 is partial: menu, tables, staff, charges and refunds are done; reports and audit review come in M5. ADM-006 stale/offline handling is partial and audited in MVP-18. Current screens are functional but unstyled until UI-01. Runtime project: `src/admin/`. This is a distinct Next.js application, not a route group inside the customer PWA.

Visual design and layout: [UI design](../product/ui-design.md) (UI-002).

## Requirements

- **ADM-001** Staff sign in to the admin origin and see workspaces permitted by their assigned roles. Go enforces the same role/resource restrictions on every direct API call. The customer PWA contains no staff workspaces.
- **ADM-002** Host/server workspaces expose the queue/table board, call/no-show/seating/move/depart/cleaning actions, assisted orders, and assistance requests from QUE/SEA/ORD requirements.
- **ADM-003** Kitchen workspace exposes active order lines and permitted preparation transitions from ORD requirements; it cannot confirm payment or edit membership.
- **ADM-004** Cashier workspace resolves dining QR tokens, reviews bills, begins/reopens/confirms settlement, and retrieves receipts from BIL requirements. Refund controls require manager authorization.
- **ADM-005** Manager workspace supports menu/table/policy configuration, staff accounts/roles, full refund recording, daily reports, and audit review from MEN/OPS/BIL requirements.
- **ADM-006** Staff screens label stale data, stop hidden-tab polling, and disable unsafe mutations when offline. Admin has no service worker in MVP; the PWA worker must never intercept admin pages or session traffic.

## Acceptance scenarios

| ID | Given / When / Then |
| --- | --- |
| ADM-A1 | Given a kitchen-only staff account, when opening cashier routes or calling settlement directly, then UI access and Go authorization both deny it (ADM-001/003/004). |
| ADM-A2 | Given the PWA installed with a service worker, when staff use the admin origin, then no PWA-controlled response or customer cookie serves the admin session (ADM-006). |
| ADM-A3 | Given the appropriate role, when staff operate a complete visit, then queue, kitchen, and cashier workspaces act on the same Go-managed visit/bill (ADM-002–004). |
| ADM-A4 | Given a revoked role or an offline admin tab, when staff attempt an unsafe action, then the interface reports the failure and the API does not authorize the revoked operation (ADM-001/006). |

Domain behavior remains in the existing feature specs; this file defines its application/workspace placement without duplicating financial or queue rules.
