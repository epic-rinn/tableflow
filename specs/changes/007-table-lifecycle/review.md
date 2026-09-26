# Review: 007-table-lifecycle

Date: 2026-09-26 (M1 milestone gate, ADR-0004). Reviewer: Claude — **self-review**. Scope: move/depart/close-empty/ready/rotate in `internal/seating/lifecycle.go`, host table actions. Result: **pass**.

## Findings

| Severity | Location | Trigger / evidence | Impact | Resolution |
| --- | --- | --- | --- | --- |
| P3 (follow-up) | `CloseEmpty` | No chargeable-order check yet (orders arrive in MVP-09) | An open visit could be closed empty after ordering | Owned by MVP-09, which must add the check and a test |
| P3 (accepted) | Host UI | "Depart" is offered for every occupied table; the API rejects unpaid visits with a clear message | Minor UX friction | Revisit when MVP-12 exposes paid state on the board |

Checked: moves keep the dining capability (SEA-A2), send the old table to cleaning and reject occupied or incompatible destinations; paid visits keep their table (SEA-A3); departure revokes dining access; rotation invalidates old sessions and QR (ACC-A2) and is audited; locking follows tables in ID order, then the visit.

## Verification

| Command / test | Result |
| --- | --- |
| `make verify` (final, 2026-09-26; macOS arm64, Go 1.27.1, Node 24.21.0, pnpm 12.6.0, postgres:18.6, Chrome 153) | **passed**: all Go tests (race, PostgreSQL), admin 8/8 and PWA 12/12 browser tests, artifact check (2715 files / 75 sentinels) |
| First M1 gate run | 1 Go failure (TestQueueJoinIdempotent; P2 below, fixed), then 1 admin browser failure (test locator), both re-run green |
| MVP-07 Go tests | TestMoveKeepsAccessAndCleansOldTable, TestRotateAccessInvalidatesOldQR, TestMoveVersusDepartOneWinner (5 rounds), TestPaidVisitKeepsTable, TestCloseEmptyAndReady: passed; paid state uses DB fixtures until MVP-12 |

## Delivery decision

No open P0/P1. Status: **done** (M1 gate, 2026-09-26).
