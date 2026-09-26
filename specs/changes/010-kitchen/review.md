# Review: 010-kitchen

Date: 2026-09-26 (M2 milestone gate, ADR-0004). Reviewer: Claude — **self-review**. Scope: kitchen transitions, assistance, admin `/kitchen` and assistance board, PWA assistance. Result: **pass**.

## Findings

No new findings beyond those in the [008](../008-menu/review.md) and [009](../009-ordering/review.md) reviews. Checked:
- the role/state matrix, including forbidden steps and roles;
- rejection and cancellation need a reason, remove the charge and bump `bill_version`;
- late cancellation is manager-only and audited;
- paid visits refuse financial edits but allow kitchen progress;
- duplicate outstanding assistance requests are merged, acknowledgement is visible to the guest, and the allergy wording makes no safety claim.

## Verification

| Command / test | Result |
| --- | --- |
| `make verify` (final, 2026-09-26T16:13Z; macOS arm64, Go 1.27.1, Node 24.21.0, pnpm 12.6.0, postgres:18.6, Chrome 153) | **passed**: 120 Go tests (race, PostgreSQL, none skipped), admin 10/10 and PWA 14/14 browser tests, artifact check (2744 files / 88 sentinels) |
| Earlier M2 gate runs | Run 1: all ordering tests failed on an invalid test menu (max 2 choices, 1 option; correctly rejected by validation), fixture fixed. Run 2: sold-out checkbox did not reflect clicks. Run 3: reloading the dining page lost access. All fixed and re-run |
| MVP-10 tests | TestLineTransitionMatrix, TestRejectedLineExcludedFromTotal (ORD-A5), TestLateCancellationNeedsManager, TestPaidVisitRejectsFinancialChanges, TestAssistanceCoalescesAndAcknowledges (ORD-A6), TestKitchenBoardPagination; admin `kitchen.spec.ts` (assisted order through the full workflow), PWA allergy test: passed |
| Plans and load | Kitchen and assistance boards against 1M historical lines and 50k requests: 0.2 / 0.1 ms; kitchen poll p95 6.4 ms under load |

## Delivery decision

No open P0/P1. Status: **done** (M2 gate).
