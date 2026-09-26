# Change: 010-kitchen — Kitchen and assistance

Status: done (M2 gate). Date: 2026-09-26. Scope owner: Claude. Task: MVP-10. Verification: M2 gate (ADR-0004).

## Problem and behavior

Maps ORD-004, ORD-006, ORD-007, ADM-002 and ADM-003.

**Kitchen board:** `GET /branches/{id}/kitchen-lines`, for kitchen, host and manager. It shows active lines (submitted, accepted, preparing, ready) oldest first, with the table label, using a partial index.

**Line transitions:** `POST /order-lines/{id}/transition {expected_version, to_state, reason?}` follows this matrix:

| From | To | Roles | Reason |
| --- | --- | --- | --- |
| submitted | accepted | kitchen, manager | — |
| submitted | rejected | kitchen, manager | required |
| accepted | preparing | kitchen, manager | — |
| preparing | ready | kitchen, manager | — |
| ready | served | kitchen, host, manager | — |
| submitted, accepted | cancelled | host, manager | required |
| preparing, ready, served | cancelled | manager only | required, audited |

- **Financial rule (ORD-007):** rejection and cancellation remove the charge and bump `bill_version`, and are allowed only while the visit is `open` (`VISIT_STATE_CONFLICT` otherwise). Progress transitions don't touch money and are allowed on paid visits.
- **Locks:** each transition locks the visit, then the line (visit → child order).

**Assistance:**
- Guests (PWA) or servers raise requests with `POST /visits/{id}/assistance {topic: help|allergy|checkout, note?}`. A partial unique index keeps one outstanding request per visit and topic; a duplicate returns the existing one with 200.
- Staff work from a board, `GET /branches/{id}/assistance`, and move requests with `POST /assistance/{id}/transition` (to `acknowledged` or `resolved`).
- Guests see their visit's requests at `GET /visits/{id}/assistance`. Allergy requests stay visibly unacknowledged until reviewed, and no text claims an item is safe.

**Admin:** `/kitchen` board with transitions and sold-out toggles; an assistance board and visit orders (assisted ordering, line cancellation) on `/host`.

## Verification map

| Requirement | Test | Status |
| --- | --- | --- |
| ORD-004 role/state matrix | TestLineTransitionMatrix | passed (M2 gate) |
| ORD-A5 rejected line not chargeable | TestRejectedLineExcludedFromTotal | passed (M2 gate) |
| Manager-only late cancellation, audited | TestLateCancellationNeedsManager | passed (M2 gate) |
| ORD-007 paid visit: no financial edits | TestPaidVisitRejectsFinancialChanges | passed (M2 gate) |
| ORD-006 coalescing, ORD-A6 acknowledgement | TestAssistanceCoalescesAndAcknowledges | passed (M2 gate) |
| Kitchen/assistance board plans vs history | Gate measurement | measured |
