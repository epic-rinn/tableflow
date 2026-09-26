# Review: 009-ordering

Date: 2026-09-26 (M2 milestone gate, ADR-0004). Reviewer: Claude — **self-review**. Scope: ordering migration, `internal/ordering` submission and reads, close-empty guard, admin visit orders, PWA dining. Result: **pass**.

## Findings

| Severity | Location | Evidence | Impact | Resolution |
| --- | --- | --- | --- | --- |
| P2 | `src/pwa/components/QrEntry.tsx` | Gate run: reloading `/t` (token already cleared from the URL) showed "scan again" although the guest session was valid | Diners lost their table page on refresh or app reopen | Fixed: without a fragment the page resumes the existing guest session of the same kind; covered by `dining.spec.ts` (cart survives reload) |
| P3 | pgx array encoding (found in implementation review) | Batch insert first used `[]any` and jsonb arrays | Possible encoding failures | Fixed before tests ran: typed slices, options cast from text |

Checked: all-or-nothing validation under a visit lock and item `FOR SHARE` locks; menu changes racing submissions give a clean accept or reject (5 rounds); retries return the original order with no extra kitchen lines; same key with a different body conflicts; bounds hold; other visits get 404; the wrong origin gets 403; reads are private; assisted orders record the staff member; only open visits accept orders; close-empty is refused with chargeable orders. The order read uses 5 statements regardless of order count (tracer test).

## Verification

| Command / test | Result |
| --- | --- |
| `make verify` (final, 2026-09-26T16:13Z; macOS arm64, Go 1.27.1, Node 24.21.0, pnpm 12.6.0, postgres:18.6, Chrome 153) | **passed**: 120 Go tests (race, PostgreSQL, none skipped), admin 10/10 and PWA 14/14 browser tests, artifact check (2744 files / 88 sentinels) |
| Earlier M2 gate runs | Run 1: all ordering tests failed on an invalid test menu (max 2 choices, 1 option; correctly rejected by validation), fixture fixed. Run 2: sold-out checkbox did not reflect clicks. Run 3: reloading the dining page lost access. All fixed and re-run |
| MVP-09 tests | TestTwoPhonesOrderOnce (ORD-A1), TestOrderRetryAndConflict (ORD-A2, 5 parallel), TestStaleMenuRejectsWholeOrder (ORD-A3), TestMenuChangeVersusSubmission, TestOrderValidation, TestOrderAccessControl, TestAssistedOrderRecordsStaff, TestOrdersOnlyOnOpenVisits, TestOrderReadStatementCount, TestOrderingContractConformance; PWA `dining.spec.ts` (distinct carts, reload, shared orders): passed |
| Load | [performance](performance.md): about 47 req/s, all 200/201, orders p95 19.6 ms |

## Delivery decision

No open P0/P1. ORD-A4 (settlement racing submission) needs MVP-12's settlement, which must lock the same visit row. Status: **done** (M2 gate).
