# Change: 009-ordering — Shared-visit ordering

Status: in-progress. Date: 2026-09-26. Scope owner: Claude. Task: MVP-09. Verification: M2 gate (ADR-0004).

## Problem and behavior

Maps ORD-001 to ORD-003, ORD-005, order gating from ORD-007, and ADM-002.
- **Carts:** each phone keeps its own draft cart (session storage, keyed by visit).
- **Submission:** `POST /visits/{id}/orders {menu_revision, lines}` by the visit's guest (PWA) or by a host or manager (admin, "assisted order"). The staff member is recorded as the actor.
- **Transaction:** lock the visit (it must be `open`), lock the items `FOR SHARE` in ID order, and validate existence, retirement, sold out, `changed_revision ≤ menu_revision`, option membership and group bounds. Then snapshot names, options and unit amounts, insert the order and its lines in one batch, and bump `visits.bill_version`. Any failure rejects the whole submission.
- **Limits:** 1–50 lines, quantity 1–20, note ≤500 characters, body ≤64 KiB.
- **Idempotency:** scope `guest:<session>` or `staff:<id>`, operation `orders.submit`, request hash = path + canonical body. Clients keep the key after an ambiguous failure.
- **Reads:** `GET /visits/{id}/orders` shows the shared confirmed orders for the visit's guests and branch staff. It returns a cursor page (2 statements: orders page plus a batch of their lines) with line states and the chargeable total. Drafts are never shared.
- **Close-empty (MVP-07 follow-up):** now refused with `VISIT_HAS_ORDERS` when a chargeable line exists.
- **Gating:** only `open` visits accept orders. MVP-12 adds `settling`, which must lock the same visit row (ORD-A4).

## Verification map

| Requirement | Test | Status |
| --- | --- | --- |
| ORD-A1 two phones, one visit | TestTwoPhonesOrderOnce | written, not run |
| ORD-A2 lost response retry; ORD-003 key/body conflict | TestOrderRetryAndConflict | written, not run |
| ORD-A3 sold out or repriced after browsing | TestStaleMenuRejectsWholeOrder | written, not run |
| Menu change racing submission | TestMenuChangeVersusSubmission | written, not run |
| Bounds and validation | TestOrderValidation | written, not run |
| Cross-visit denial, private reads | TestOrderAccessControl | written, not run |
| Assisted order actor | TestAssistedOrderRecordsStaff | written, not run |
| Order gating on non-open visits; close-empty guard | TestOrdersOnlyOnOpenVisits | written, not run |
| Distinct carts per phone | PWA browser test | written, not run |
