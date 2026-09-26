# Change: 007-table-lifecycle — Moves and table lifecycle

Status: in-progress. Date: 2026-09-26. Scope owner: Claude. Task: MVP-07. Verification: M1 gate (ADR-0004).

## Problem and behavior

Maps SEA-003, SEA-004, ACC-001 and ADM-002.
- **Move:** a visit moves to an available compatible table, keeping its dining capability (and, later, its orders and bill); the old table goes to cleaning.
- **Depart:** only for paid visits; releases the claim, sets the table to cleaning and revokes the dining capability.
- **Close empty:** an open visit with no chargeable orders (none exist until MVP-09, which adds the check) closes with a reason and audit.
- **Ready:** a cleaning, unclaimed table becomes available.
- **Rotate access:** issues a new dining QR, invalidates every derived guest session, and is audited with a reason.

No merging or splitting of tables.

## Decisions

- **Lock order for move/depart/close:** read the visit's current table without a lock; lock the involved tables in ID order, then the visit; re-check that the visit is still on the table read earlier, otherwise return `409 VERSION_CONFLICT` so the client retries. This avoids visit→table inversion (data model).
- **Paid visits** keep their claim until departure (SEA-A3). Assigning their table fails with `TABLE_UNAVAILABLE`, tested with fixtures that set `paid` directly until MVP-12 exists.

## Routes

`POST /visits/{id}/move`, `/depart`, `/close-empty`, `/rotate-access`, `POST /tables/{id}/ready`.

## Verification map

| Requirement | Test | Status |
| --- | --- | --- |
| SEA-A2 move keeps access; old table cleaning | TestMoveKeepsAccessAndCleansOldTable | written, not run |
| ACC-A2 rotation | TestRotateAccessInvalidatesOldQR | written, not run |
| Move versus depart race | TestMoveVersusDepartOneWinner | written, not run |
| SEA-A3 paid retention | TestPaidVisitKeepsTable | written, not run |
| Close-empty, ready, occupied rejection | TestCloseEmptyAndReady | written, not run |
