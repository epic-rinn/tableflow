# Review: 005-queue-entry

Date: 2026-09-26 (M1 milestone gate, [ADR-0004](../../decisions/0004-milestone-verification.md)). Reviewer: Claude — **self-review**. Scope: M1 commits `8695f80`, `159dc6a`, `90c0f64` plus gate fixes (migration `20260926160000_queue_entry.sql`, `internal/seating` queue/config parts, admin `/configuration` and `/host` queue parts, PWA `/join`, queue tracking, polling helpers, OpenAPI). Result: **pass**.

## Findings

| Severity | Location | Trigger / evidence | Impact | Resolution |
| --- | --- | --- | --- | --- |
| P2 | `internal/seating/http.go` join | Gate: TestQueueJoinIdempotent returned 429 for the 4th of 6 identical retries: the per-session join limit ran before the idempotency check | A phone retrying after a lost response could get "too many attempts" instead of its ticket (QUE-A1) | Fixed: the limit runs inside the idempotent section, so only first executions count; re-run green |
| P2 | `internal/seating/queue.go` Cancel (found during implementation review) | A guest cancel re-validated its capability (`FOR SHARE`) before locking the ticket, while seating locks the ticket and then updates that capability | Possible deadlock between a guest cancelling and a host seating the same ticket | Fixed before commit: guests lock the ticket first, then re-validate; rule recorded in the data model |
| P3 (accepted) | Display numbering | The daily counter row must stay consistent with existing tickets (seen as a fixture defect during load testing) | After a partial or manual data restore, joins could fail with 503 until the counter is fixed | Recorded for the MVP-21 restore runbook (tickets and counters restore together) |
| P3 (accepted) | Printed QR | Assisted joins show the display number and a tracking link, not a rendered QR image | Staff read out or copy the link; printing needs a QR renderer | Follow-up MVP-18 (journey hardening) |

## Verification

| Command / test | Result |
| --- | --- |
| `make verify` (final, 2026-09-26; macOS arm64, Go 1.27.1, Node 24.21.0, pnpm 12.6.0, postgres:18.6, Chrome 153) | **passed**: all Go tests (race, PostgreSQL), admin 8/8 and PWA 12/12 browser tests, artifact check (2715 files / 75 sentinels) |
| First M1 gate run | 1 Go failure (TestQueueJoinIdempotent; P2 below, fixed), then 1 admin browser failure (test locator), both re-run green |
| MVP-05 Go tests | TestQueueJoinIdempotent (6 parallel), TestGroupPositionIndependentOfOtherGroups (QUE-A2), TestJoinOrderSurvivesDailyRenumbering (QUE-A4), TestTrackingShowsOnlyOwnTicket, TestQueueBranchIsolation, TestTableConfiguration, TestSeatingGroupValidation, TestGuestAndHostCancel, TestJoinRateLimitedPerSession, TestSeatingContractConformance: passed |
| Browser | admin `host.spec.ts`: configuration and full host journey; polling paused while hidden (0 requests in 7 s), immediate refresh on return, error backoff (≤3 polls in 12 s) and stale warning. PWA `queue.spec.ts`: join, group position wording, cancel, hidden-tab pause, oversized party. All passed |
| Load | [performance](performance.md): 53 req/s mix, all 200/201, p95 ≤12.8 ms |

## Delivery decision

No open P0/P1. Status: **done** (M1 gate, 2026-09-26).
