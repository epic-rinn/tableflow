# MVP HTTP contract

Status: design contract; health, identity, guest-session, member, queue/seating and menu/ordering/kitchen/assistance routes are implemented and their wire schemas live in [openapi.yaml](openapi.yaml). Base path: `/api/v1`. All routes below are relative to that base. Wire schemas move to OpenAPI per [architecture](../architecture/system.md) before their implementation.

## Common conventions

- JSON UTF-8. IDs are UUID strings; times are RFC 3339 UTC; dates are branch-local `YYYY-MM-DD`; money is integer satang and `currency: "THB"`. Quantities/rates are bounded integers. Reject unknown mutation fields and invalid enums.
- Go owns authentication, authorization, totals, and branch selection. Never trust a body-supplied actor, role, price, or calculated total. Guest resources derive their branch from the authenticated capability.
- `201` creates a resource, `200` reads/transitions/replays, `204` logs out. Return `400` malformed input, `401` missing/invalid identity, `403` disallowed role/CSRF, `404` inaccessible resource, `409` state/version/idempotency conflict, `422` field/business validation, `429` rate limit, `503` temporary dependency failure.
- Error shape: `{ "error": { "code": "BILL_VERSION_CONFLICT", "message": "Refresh the bill", "request_id": "...", "fields": {} } }`. Use stable codes; never expose SQL, tokens, or internal stack traces.
- Business mutations require `Idempotency-Key` (UUID) and a resource `expected_version` where specified. Same scoped key/body returns the committed original status/body; different body yields `409 IDEMPOTENCY_CONFLICT`. Authentication uses single-use tokens/normal auth retry semantics rather than business idempotency.
- All private responses use `Cache-Control: private, no-store`. Authenticate before conditional responses. Public menu responses may be cacheable only after explicit cache policy tests; MVP defaults to no-store for simplicity.
- Lists use `limit` (default 25, max 100), opaque `cursor`, stable `(created_at,id)` or domain sequence ordering. Response: `{ "items": [], "next_cursor": null, "server_time": "..." }`. Bind cursor to branch/filter/sort; reject invalid cursors. No arbitrary client SQL sorting or unlimited exports.
- Mutations return the updated resource `id`, `state`, `version`, `server_time`, plus operation-specific fields. All writes require CSRF/Origin checks when cookie-authenticated. Reads never transition state or expire holds.

## Identity

| Method and path | Actor | Input → output |
| --- | --- | --- |
| POST `/sessions/anonymous` | Public, PWA Origin, rate limited | Empty → anonymous cookie; bootstrap identity for queue join (implemented, MVP-03) |
| POST `/sessions/capability` | Public, PWA Origin, rate limited | `{token, kind: queue\|visit}` → guest cookie and resource ID/branch; token appears only in body (implemented, MVP-03; resource state added by MVP-05/06) |
| GET `/sessions/guest` | Guest session | Current guest scope (implemented, MVP-03) |
| POST `/members` | Public, PWA Origin, rate limited | `{email,password,locale}` → 202 generic acknowledgement; verification email or existing-account notice (implemented, MVP-04) |
| POST `/members/verification` | Public, rate limited | `{email}` → 202 neutral; new verification link for unverified accounts (implemented) |
| POST `/sessions/member` | Member credentials | `{email,password}` → member cookie and own identity (implemented) |
| GET `/members/me` / DELETE `/sessions/member` | Member session | Own identity / sign-out (implemented; member-specific instead of `/sessions/current`, which serves staff) |
| POST `/sessions/staff` | Staff credentials, admin Origin, throttled | `{email,password}` → 201 staff cookie, assigned branch/roles (implemented) |
| GET `/sessions/current` | Current principal | Own identity/roles (implemented for staff) |
| DELETE `/sessions/current` | Current principal | Empty → revoke current session, clear cookie (implemented for staff; guest/member in MVP-03/04) |
| POST `/staff/activate` | Manager-issued single-use token | `{token,password,display_name?}` → activates an invited staff account (implemented) |
| POST `/members/verify` | Single-use token | `{token}` → verification acknowledgement (implemented) |
| POST `/members/password-reset/request` | Public, rate limited | `{email}` → neutral acknowledgement (implemented) |
| POST `/members/password-reset/confirm` | Single-use token | `{token,new_password}` → reset, verify email and revoke old sessions (implemented) |

Staff account invitation/activation uses manager-issued single-use credentials; do not expose a public role selector. A browser may hold separately scoped member and guest sessions so a logged-in member can claim a visit without destroying its guest access.

## Queue and seating

| Method and path | Actor | Input → output / requirement |
| --- | --- | --- |
| POST `/branches/{branch_id}/queue-tickets` | Anonymous session or host | `{party_size,seating_needs?}` → ticket and queue access link/QR; QUE-001 |
| GET `/queue-tickets/{ticket_id}` | That queue guest or host | Ticket number/state/group/parties_ahead/called_until/version/time; no other guest details; QUE-002 |
| POST `/queue-tickets/{ticket_id}/cancel` | That queue guest or host | `{expected_version}` → terminal ticket; QUE-004 |
| GET `/branches/{branch_id}/queue-tickets` | Host | `state,group,limit,cursor` → board page; QUE-003 |
| POST `/queue-tickets/{ticket_id}/call` | Host; manager for bypass | `{table_id,expected_version,override_reason?}` → ticket/deadline/table hold; QUE-003 |
| POST `/queue-tickets/{ticket_id}/no-show` | Host | `{expected_version,reason}` → released hold; QUE-004 |
| POST `/visits` | Host | `{branch_id,table_id,queue_ticket_id?,expected_table_version,override_reason?}` → visit and dining access link; SEA-001 |
| GET `/branches/{branch_id}/tables` | Host/cashier | Bounded board (max 100 tables in pilot) with capacity/state/claim/version; SEA-002 |
| POST `/visits/{visit_id}/move` | Host | `{table_id,expected_version,expected_table_version}` → preserved visit with new table; SEA-003 |
| POST `/visits/{visit_id}/depart` | Host | `{expected_version}` → paid visit's claim released, table cleaning; SEA-004 |
| POST `/visits/{visit_id}/close-empty` | Host | `{expected_version,reason}` → close visit only with no chargeable orders, release claim to cleaning; SEA-004 |
| POST `/tables/{table_id}/ready` | Host | `{expected_version}` → available if cleaning and unclaimed; SEA-004 |
| POST `/visits/{visit_id}/rotate-access` | Host | `{expected_version,reason}` → new QR; old derived guest sessions revoked; ACC-001 |

Capability-authenticated tracking after seating reports the terminal queue state; it does not reveal the dining secret. Staff give the separate dining QR at the table.

Implemented in M1 (MVP-05–07) with these additions: GET/PUT `/branches/{branch_id}/seating-groups` (seating part of configuration), GET `/visits/{visit_id}` for the visit's guests or staff, `POST /queue-tickets/{id}/cancel` also releases a hold, and `POST /visits` accepts `party_size`/`needs` for walk-ins. Wire schemas: [openapi.yaml](openapi.yaml).

## Menu, orders, kitchen, assistance

| Method and path | Actor | Input → output / requirement |
| --- | --- | --- |
| GET `/branches/{branch_id}/menu` | Public | `locale` → revision/categories/items/options/prices/availability; MEN-001 |
| GET `/visits/{visit_id}` | That visit guest or staff | State/table display/version; no member PII; ORD-005 |
| POST `/visits/{visit_id}/orders` | That visit guest or server | `{menu_revision,lines:[{item_id,quantity,option_ids,note?}]}` → order and immutable lines; ORD-001/002/003 |
| GET `/visits/{visit_id}/orders` | That visit guest or staff | Cursor page with batched lines/state; ORD-005 |
| GET `/branches/{branch_id}/kitchen-lines` | Kitchen/server | `state,limit,cursor` → active line page with order/table context; ORD-004 |
| POST `/order-lines/{line_id}/transition` | Kitchen/server/manager by transition | `{expected_version,to_state,reason?}` → line; enforce role/state matrix from ORD-004/007 |
| POST `/visits/{visit_id}/assistance` | That visit guest/server | `{topic}` → unresolved request, coalescing duplicates; ORD-006 |
| GET `/branches/{branch_id}/assistance` | Server | `state,limit,cursor` → assistance board |
| POST `/assistance/{request_id}/transition` | Server | `{expected_version,to_state: acknowledged\|resolved}` → request |

Implemented in M2 (MVP-08–10). `PUT /branches/{id}/menu` takes `expected_revision` and a nested category → item → group → option tree; `PATCH /menu-items/{id}/availability` uses the item version; `GET/POST /visits/{id}/assistance` serve the visit's guests. `GET /branches/{id}/menu?category_id=` returns all categories with items for one category only (large-menu split); the menu route is gzip-compressed when accepted. Wire schemas: [openapi.yaml](openapi.yaml).

Menu responses are capped at the pilot's configured maximum 500 items, with bounded options per item. M0/ordering contract tests must validate a response-byte cap; do not silently truncate a menu. Split by category if the pilot exceeds that bound.

## Billing and loyalty

| Method and path | Actor | Input → output / requirement |
| --- | --- | --- |
| GET `/visits/{visit_id}/bill` | That visit guest/cashier | Itemized bill, totals, version, anonymous claim status; BIL-001/003 |
| POST `/bills/resolve` | Cashier | `{dining_token}` → visit/bill; token in body, no guest login required; BIL-001 |
| POST `/visits/{visit_id}/settlement/begin` | Cashier | `{expected_version}` → settling snapshot/totals/version; BIL-002 |
| POST `/visits/{visit_id}/settlement/reopen` | Cashier | `{expected_version,reason}` → open/new version; BIL-007 |
| POST `/visits/{visit_id}/settlement/confirm` | Cashier | `{expected_version,amount_satang,method,verification_note,external_reference?}` → settlement/receipt; BIL-005/006 |
| POST `/settlements/{settlement_id}/refund` | Manager | `{reason,external_reference}` → one full refund/reversal; BIL-007 |
| GET `/settlements/{settlement_id}` | Cashier/manager | Historical receipt/snapshot; BIL-008 |
| GET `/branches/{branch_id}/settlements` | Cashier/manager | `?limit,cursor,receipt_reference` → receipts newest first, keyset `(paid_at,id)`; BIL-008 (MVP-13) |
| POST `/visits/{visit_id}/member-claim` | Member plus matching visit guest access | `{expected_version}` → anonymous claim status and incremented bill version; LOY-001 |
| POST `/visits/{visit_id}/member-detach` | Cashier/manager, open visit only | `{expected_version,reason}` → detached claim; LOY-002 |
| GET `/members/me/loyalty` | Member | Own points/tier/qualifying spend/next threshold; LOY-007 |
| GET `/members/me/loyalty/entries` | Member | Cursor → own ledger history; LOY-007 |
| GET `/visits/{visit_id}/member-claim` | Member plus matching visit guest access | `{claimed, mine, tier?, discount_bp?, bill_version, visit_state}`; tier/discount only when mine; LOY-001 (MVP-14) |
| GET/PUT `/branches/{branch_id}/loyalty-policy` | Staff read / manager write | Versioned points rate and Silver/Gold thresholds/discounts; version 0 = pilot defaults (MVP-14) |

Payment confirmation authenticates staff again on every retry, even if the idempotency result exists. After guest access is revoked at payment, receipt retrieval remains staff-mediated in MVP.

Settlement conflicts use stable codes: `BILL_VERSION_CONFLICT` (body also carries the fresh `bill`), `UNRESOLVED_LINES` (body carries `lines`), `ALREADY_PAID` (body carries `settlement`, `fields.receipt_reference`), `VISIT_STATE_CONFLICT`, `NOTHING_TO_SETTLE` (no chargeable lines; use close-empty), `ALREADY_CLAIMED` / `NOT_CLAIMED` (member claims), `AMOUNT_MISMATCH` (422) and `ALREADY_REFUNDED`. The visit gains the `settling` state between begin and confirm/reopen.

## Configuration and reports

Manager-only, branch-authorized routes:

- GET `/branches/{branch_id}/configuration` → current seating/charge/loyalty policies and versions.
- PUT `/branches/{branch_id}/configuration` with `{expected_version,seating,charges,loyalty}` → validated new policy version; historical snapshots unchanged.
- Implemented as separately versioned resources: seating groups (`/branches/{branch_id}/seating-groups`, MVP-06) and charges (GET/PUT `/branches/{branch_id}/charge-policy` with `{expected_version,tax_mode,tax_bp,service_bp}`, rates in basis points, append-only versions, MVP-11). Loyalty policy is `/branches/{branch_id}/loyalty-policy` (MVP-14). GET charge-policy is readable by branch staff; PUT is manager-only.
- POST `/branches/{branch_id}/tables` and PATCH `/tables/{table_id}` → label/capacity/needs/active changes; forbid disabling/changing an actively claimed table incompatibly.
- PUT `/branches/{branch_id}/menu` with `{expected_version,categories,items,option_groups,options}` → atomic bounded menu revision; use IDs to retain history, no destructive replacement of historical references. Maximum configuration body 1 MiB, manager routes only.
- PATCH `/menu-items/{item_id}/availability` with `{expected_version,sold_out}` → revision bump.
- GET/POST `/branches/{branch_id}/staff` → paginated accounts / create invitation with one-time activation token; POST `/staff/{staff_id}/activation` → replace an invited account's token; PATCH `/staff/{staff_id}/roles` with `{expected_version,roles}` → role update/session revocation; POST `/staff/{staff_id}/deactivate` with `{expected_version,reason}` → disable and revoke. Implemented in MVP-02; staff-admin writes use `expected_version` and natural uniqueness until the idempotency infrastructure of MVP-03 exists.
- GET `/branches/{branch_id}/reports/daily` with `from,to` (max 31 days) → aggregates; GET `/branches/{branch_id}/audit-events` with bounded date range/cursor → redacted events. Implemented in MVP-16: business dates in the branch timezone, each metric dated by its own event, optional `action` family filter on audit, and `from,to` on `/branches/{branch_id}/settlements` for report drill-down.

Before each route is implemented, its change plan must list exact request/response schemas, field constraints, role matrix, query count, indexes, errors, and mapped acceptance tests. No endpoint is delivery-ready just because it appears in this inventory.
