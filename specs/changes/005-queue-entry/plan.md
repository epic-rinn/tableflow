# Change: 005-queue-entry — Tables and queue entry

Status: in-progress. Date: 2026-09-26. Scope owner: Claude. Task: MVP-05. Verification: M1 milestone gate ([ADR-0004](../../decisions/0004-milestone-verification.md)).

## Problem and behavior

Maps QUE-001, QUE-002, waiting cancellation from QUE-004, the table subset of OPS-001/ADM-005, and ADM-002 ([queue/seating](../../features/01-queue-seating.md)).

- **Managers** configure tables (label, capacity, supported needs, active) and seating groups (party-size bands).
- **Guests** join the queue from the entrance QR (`/join/<branch_id>`) and are redirected to their tracking link (`/q#token`). They can see their state and position within their group, and cancel.
- **Hosts** can join a party on its behalf (the ticket number and QR are shown for printing or reading out) and see the waiting board.

## Decisions (reversible unless noted)

- **Seating needs:** a fixed vocabulary, `accessible` and `high_chair`. Tables list the needs they support; a ticket is compatible with a table when `party_size ≤ capacity` and all of its needs are supported.
- **Seating groups:** `[min_party, max_party]` bands, contiguous from 1 without overlap; the default is 1–2, 3–4, 5–6. `PUT /branches/{id}/seating-groups` replaces the set only while no ticket is waiting or called (`409 QUEUE_ACTIVE`), so every active ticket's group position stays meaningful. Guests whose party is larger than the largest band get `422 PARTY_NEEDS_STAFF`; hosts may join such parties with no group, and those tickets are shown to staff as "special".
- **Numbering:** the display number comes from a `queue_counters` row per branch and business date (the branch's time zone, Asia/Bangkok by default). `join_order` is a global identity sequence and alone decides priority, so a daily reset never lets new arrivals overtake yesterday's waiting parties (QUE-A4).
- **Joining:** a guest join needs the anonymous session and an `Idempotency-Key`, with scope `anon:<session>`. The response carries the tracking token, stored sealed for replay (QUE-A1). Limits: 5 joins per anonymous session and 30 per IP per 10 minutes. A host join uses scope `staff:<id>` and needs the host or manager role.
- **Tracking:** a `queue` capability for the ticket. `GET /queue-tickets/{id}` answers the matching guest session, or staff of the branch; anyone else gets 404. It shows the display number, state, group label, `parties_ahead` (waiting tickets in the same group with an earlier `join_order`), `called_until`, the table label when called, version and server time, and never other diners' data. It is labelled "position in your group".
- **Board:** `GET /branches/{id}/queue-tickets?state=waiting|called` lists active tickets only, keyset-paged by `join_order` (limit ≤100). It uses a partial index on active states, so historical tickets are never scanned.
- **Cancellation:** `POST /queue-tickets/{id}/cancel {expected_version}` by the ticket's guest or by staff; waiting tickets only in MVP-05, while called-ticket cancellation (which releases the hold) arrives in MVP-06. The capability stays valid for 2 hours after a terminal state, so tracking can show the outcome.
- **Business mutations** use `Idempotency-Key` and re-validate the actor inside the transaction. Lock order: actor rows → queue tickets → tables → visits (the data-model order).
- **Polling:** the admin board polls every 3 s and guest tracking every 10 s, both ±20% jitter. Polling pauses in hidden tabs, backs off exponentially on errors, fetches immediately on focus or reconnect, and shows a "last updated" time plus a stale warning.

## Routes

| Route | Actor |
| --- | --- |
| GET/PUT `/branches/{id}/seating-groups` | Staff read / manager write |
| GET/POST `/branches/{id}/tables`, PATCH `/tables/{id}` | Staff read (board) / manager write |
| POST `/branches/{id}/queue-tickets` | Anonymous session (PWA) or host (admin), `Idempotency-Key` |
| GET `/branches/{id}/queue-tickets` | Host/manager board |
| GET `/queue-tickets/{id}` | Ticket guest or branch staff |
| POST `/queue-tickets/{id}/cancel` | Ticket guest or host, `Idempotency-Key` |

## Verification map

| Requirement | Test | Status |
| --- | --- | --- |
| QUE-A1 same-key join retry (parallel) | TestQueueJoinIdempotent | written, not run |
| QUE-A2 group position | TestGroupPositionIndependentOfOtherGroups | written, not run |
| QUE-A4 priority across the date boundary | TestJoinOrderSurvivesDailyRenumbering | written, not run |
| Tracking privacy and branch isolation | TestTrackingShowsOnlyOwnTicket, TestQueueBranchIsolation | written, not run |
| Table/group configuration and validation | TestTableConfiguration, TestSeatingGroupValidation | written, not run |
| Cancellation | TestGuestAndHostCancel | written, not run |
| Hidden-tab/backoff polling | admin/PWA browser tests | written, not run |
