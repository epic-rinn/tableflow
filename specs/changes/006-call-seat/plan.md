# Change: 006-call-seat — Calling and seating

Status: implemented-unverified. Date: 2026-09-26. Scope owner: Claude. Task: MVP-06. Verification: M1 gate (ADR-0004).

## Problem and behavior

Maps QUE-003, QUE-004, SEA-001, SEA-002 and ADM-002. Hosts call a waiting ticket to a specific table: an atomic hold with a return deadline. They mark a no-show, cancel a called ticket (releasing the hold), or seat the party, creating a visit and a dining QR. Direct seating covers a waiting ticket without a call, or a walk-in given a party size and needs. Bypassing an older compatible waiting party needs the manager role and a reason, and is audited.

## Decisions

- **Exclusivity:** `table_claims` has `table_id` as primary key and exactly one owner, either `queue_ticket_id` or `visit_id` (each unique). It is the exclusivity mechanism for holds and visits (SEA-002).
- **Call:** lock the ticket (must be waiting), then the table (must be available, active and compatible). Check fairness: an older compatible waiting ticket makes this a bypass. Insert the claim, set the ticket to `called` with `called_until = now + hold minutes` (branch setting, default 5) and the table to `held`.
- **Deadlines:** they never expire holds by themselves. The board flags overdue tickets and staff mark a no-show explicitly (QUE-004).
- **No-show / called cancel:** lock the ticket, then its table; delete the claim, return the table to `available`, and set the ticket to `no_show` or `cancelled`. A seat racing a no-show has one winner through the ticket row lock and state checks (QUE-A3).
- **Seat:** lock the ticket if present, then the table. A ticket called to this table converts its claim to the new visit. A waiting ticket or a walk-in goes through the same fairness check. Seating inserts the visit (party size, needs, table), creates the dining capability (returned in the response and sealed in the idempotency store), and sets the table to `occupied` and the ticket to `seated`. Retrying the same key returns the original visit and QR.
- **Visit states in M1:** `open`, `paid` (set by the cashier in MVP-12; database fixtures until then), `departed`, `closed`. `GET /visits/{id}` serves the visit's guest (dining capability) or branch staff and returns state, table label, party size and version.
- **Losers:** they receive `409 TABLE_UNAVAILABLE`, `TICKET_STATE_CONFLICT` or `VERSION_CONFLICT`; clients refresh the board.

## Routes

`POST /queue-tickets/{id}/call`, `/no-show`, `/cancel` (called), `POST /visits`, `GET /visits/{id}`, `GET /branches/{id}/tables` (board with claims).

## Verification map

| Requirement | Test | Status |
| --- | --- | --- |
| SEA-A1 two hosts, one table (call and seat) | TestConcurrentCallsOneClaim, TestConcurrentSeatsOneVisit | written, not run |
| QUE-A3 no-show versus seat | TestNoShowVersusSeatOneWinner | written, not run |
| Repeated seating returns the original visit | TestSeatRetryReturnsOriginal | written, not run |
| Deadline does not release the hold | TestOverdueHoldPersists | written, not run |
| Fairness and manager override with audit | TestBypassRequiresManagerReason | written, not run |
| Incompatible table rejected | TestIncompatibleTableRejected | written, not run |
