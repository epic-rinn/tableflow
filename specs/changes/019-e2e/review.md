# Review: 019-e2e

Date: 2026-09-27 (M6 milestone gate, ADR-0004). Reviewer: Claude — **self-review**. Scope: cross-app journey config and spec, `make journey` in `make verify`, requirement citations and coverage matrix, concurrency group 4 test. Result: **pass**.

## Findings

| Severity | Location | Evidence | Impact | Resolution |
| --- | --- | --- | --- | --- |
| P3 | `journey/visit.spec.ts` | First runs: Next's route announcer is also `role=alert`; the verify link was relative; leftover waiting parties made fairness require an override | Test-only failures | Scoped locators to `main`, prefixed the PWA origin, and had the host clear leftover tickets (fairness correctly enforced) |
| P3 | `host.spec.ts` polling window | One run saw 1 poll in 7 s: 3 s ±20% jitter allows 7.2 s for two polls | Flaky assertion | The window is now 8 s |

**Checked:**
- **The journey (ADM-A3):** it drives both apps against one visit and bill.
  - Two phones keep separate carts, and phone A never sees the member's email.
  - Payment revokes dining access.
  - Points appear in the member's history.
  - The table passes through paid → departed → cleaning → ready.
- **Coverage:** 81/81 requirement and acceptance IDs are cited by tests; all seven concurrency groups map to real PostgreSQL tests with barriers.
- **New test:** `TestCancellationVersusBegin` passes over 5 rounds, twice, under `-race`.

## Verification

| Command / test | Result |
| --- | --- |
| `make verify` (final, 2026-09-26T22:58Z UTC; macOS arm64, Go 1.27.1, Node 24.21.0, pnpm 12.6.0, postgres:18.6, Chrome) | **passed**: 162 Go tests (race, PostgreSQL, none skipped), admin 16/16, PWA 25/25, cross-app journey 1/1, artifact check (2,881 files / 145 sentinels, no leaks) |
| Coverage | [coverage.md](coverage.md) |

## Delivery decision

No open P0/P1. Status: **done** (M6 gate).
