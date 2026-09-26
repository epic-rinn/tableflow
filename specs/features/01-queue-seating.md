# Queue and seating

Status: specified; implemented and verified in M1 (MVP-05–07, gate passed 2026-09-26). Actors: guest, host/server, manager.

## Requirements

- **QUE-001** Joining creates one ticket for the branch/business date, party size, seating group, and optional seating requirements; it returns a personal tracking capability. Display numbers are unique per branch/date. A separate monotonic join order controls priority across midnight; resetting display numbers cannot put new arrivals ahead of yesterday's waiting parties. Staff can create tickets and provide a printed/retrieval alternative.
- **QUE-002** Tracking shows state, server update time, and waiting parties ahead in the same seating group ordered by join sequence. Label this as group position, not guaranteed global seating order. Special requirements are shown to staff; no wait-time prediction in MVP.
- **QUE-003** Host calls a waiting ticket against a specific available, compatible table. Calling atomically creates a table hold and return deadline. Staff see oldest compatible candidates; bypassing an older compatible waiting ticket requires a manager reason.
- **QUE-004** Cancellation or host-confirmed no-show releases the hold. A past deadline is highlighted, but does not silently expire a hold; staff explicitly mark no-show to avoid races with arriving guests. Rejoining creates a new ticket at the end.
- **SEA-001** Seating consumes a called ticket/hold, creates one visit and dining capability, and marks the table occupied in one transaction. Direct seating is allowed only if it does not bypass a compatible waiting party, unless a manager records a reason.
- **SEA-002** Only one active claim can exist for a table. Concurrent call/seat/move commands have one winner; losers receive a conflict and refreshed state. Retrying the same successful command returns the original result.
- **SEA-003** A move changes the existing visit's table claim, preserves orders/bill/access, and puts the old table into cleaning. The destination must be available and compatible. No merging/splitting occupied tables.
- **SEA-004** Paid visits keep their table claim until staff mark departure. Departure releases the claim and sets cleaning; a separate ready action makes the table available. Empty cancelled visits require a staff close action and audit reason.

Queue transitions: `waiting → called → seated`; waiting/called may become `cancelled`; called may become `no_show`. Terminal tickets cannot be called or seated again. Table states: `available → held → occupied → cleaning → available`; hold cancellation returns to available.

## Acceptance scenarios

| ID | Given / When / Then |
| --- | --- |
| QUE-A1 | Given a retried join request with the same key/body, when both reach the API, then one ticket and one tracking capability result (QUE-001). |
| QUE-A2 | Given groups 1–2 and 3–4, when a two-seat table is free, then staff can call the oldest eligible 1–2 ticket without moving a four-person party's group position (QUE-002/003). |
| QUE-A3 | Given a called ticket whose deadline passed, when staff mark no-show, then the hold releases; a concurrent seat attempt cannot also succeed (QUE-004, SEA-002). |
| QUE-A4 | Given a party still waiting across the branch's date boundary, when daily display numbering resets, then its priority remains ahead of newly joined compatible parties (QUE-001/002). |
| SEA-A1 | Given two hosts targeting the same table, when both call or seat, then exactly one active claim and at most one new visit exist (SEA-001/002). |
| SEA-A2 | Given an occupied visit, when moved, then the original dining QR retrieves the same bill and the old table needs cleaning (SEA-003). |
| SEA-A3 | Given a paid party still seated, when another host tries to assign its table, then assignment fails until departure and cleaning are recorded (SEA-004). |

## Reads and writes

See [HTTP contract](../api/http.md) and [data model](../architecture/data-model.md). Queue polling must not scan historical closed tickets. Board requests batch table/ticket data instead of fetching each table separately. Test branch isolation even with a one-branch UI.
