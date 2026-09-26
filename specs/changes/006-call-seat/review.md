# Review: 006-call-seat

Date: 2026-09-26 (M1 milestone gate, ADR-0004). Reviewer: Claude — **self-review**. Scope: migration `20260926170000_call_seat.sql`, call/no-show/seat/visit code in `internal/seating`, host queue actions. Result: **pass**.

## Findings

| Severity | Location | Trigger / evidence | Impact | Resolution |
| --- | --- | --- | --- | --- |
| P3 (open decision) | Fairness rule (`older_compatible.sql`) | Any older waiting party that *fits* a table counts as compatible, so seating a party of 6 at the only 6-top while an older couple waits needs a manager reason | More manager overrides than hosts may expect | Follows QUE-003/SEA-001 literally; the pilot operator should confirm or refine it (MVP-22). The override is available and audited |
| P3 (accepted) | Idempotency retention | Seating replays are kept 72 h (the data model says 24 h and until the resource ends) | Tickets and visits end within a business day, so 72 h covers both | Documented in `http.go` |

Checked: one claim per table under racing calls and seats; no-show versus seat has one winner with consistent ticket, table and visit state; retries return the original visit and dining token (stored sealed, not in plaintext); overdue holds persist until staff act; incompatible tables are rejected; bypasses need a manager and a reason and are audited; other branches get 404.

## Verification

| Command / test | Result |
| --- | --- |
| `make verify` (final, 2026-09-26; macOS arm64, Go 1.27.1, Node 24.21.0, pnpm 12.6.0, postgres:18.6, Chrome 153) | **passed**: all Go tests (race, PostgreSQL), admin 8/8 and PWA 12/12 browser tests, artifact check (2715 files / 75 sentinels) |
| First M1 gate run | 1 Go failure (TestQueueJoinIdempotent; P2 below, fixed), then 1 admin browser failure (test locator), both re-run green |
| MVP-06 Go tests | TestConcurrentCallsOneClaim (5 rounds), TestConcurrentSeatsOneVisit (5 rounds), TestNoShowVersusSeatOneWinner (QUE-A3, 5 rounds), TestSeatRetryReturnsOriginal, TestOverdueHoldPersists, TestBypassRequiresManagerReason, TestIncompatibleTableRejected, TestCancelCalledReleasesHold: passed |

Concurrency tests use independent connections behind a start barrier. Unlike MVP-02's lock-order test, their interleaving is not forced, so they show invariants under real races but do not prove every interleaving.

## Delivery decision

No open P0/P1. Status: **done** (M1 gate, 2026-09-26).
