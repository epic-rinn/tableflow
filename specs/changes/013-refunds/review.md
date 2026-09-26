# Review: 013-refunds

Date: 2026-09-26 UTC (M3 milestone gate, ADR-0004). Reviewer: Claude — **self-review**. Scope: `refunds`, receipt list and detail, manager refund, and the admin receipts UI. Result: **pass**.

## Findings

No P0–P2 findings.

| Severity | Location | Evidence | Impact | Resolution |
| --- | --- | --- | --- | --- |
| P3 | Receipt reference generation | 50 random bits, unique constraint, no retry on collision | A collision (negligible at pilot volume) would return 503 and need a retry | Accepted; revisit only if volume grows by orders of magnitude |

Checked:
- **Lock order:** visit → settlement → refund. Three concurrent refunds plus a replay leave one refund row and one audit event; the others get `ALREADY_REFUNDED` (non-member part of BIL-A6).
- **Roles:** cashier-only staff get 403 and anonymous callers are refused. Empty inputs get 422.
- **Immutability:** the settlement, snapshot lines, amount and receipt reference are unchanged after a refund and after later menu and policy changes. The visit stays paid; begin and reopen return `ALREADY_PAID`.
- **Receipt lookup:** branch-scoped keyset pagination, exact reference filter, 404 for other branches, 403 for kitchen.

## Verification

| Command / test | Result |
| --- | --- |
| `make verify` (final, 2026-09-26T18:23Z UTC; macOS arm64, Go 1.27.1, Node 24.21.0, pnpm 12.6.0, postgres:18.6, Chrome) | **passed**: 142 Go tests (race, PostgreSQL, none skipped), admin 11/11 and PWA 15/15 browser tests, artifact check (2783 files / 100 sentinels) |
| Earlier M3 gate runs | Run 1 (`api-test-db`): route-parity test lacked the billing provider; guest→staff settlement was refused with 403 by the origin guard, where the test expected only 401; the revoked-QR check used a helper that fails on exchange. Tests fixed; product behaviour was correct. Run 2 (`make verify`): passed. Run 3, after the review fixes: the begin-versus-order race fixture had no lines and now correctly got `NOTHING_TO_SETTLE`; fixture fixed and the test repeated 5 times. Run 4: passed |
| MVP-13 tests | TestConcurrentRefundOneRecord, TestRefundRoles, TestReceiptImmutable, TestReceiptLookup, TestBillingContractConformance; admin `cashier.spec.ts` (receipt → refund → list shows Refunded) |
| Load | [performance](performance.md): receipt list p95 6.2 ms, receipt 3.6 ms over 100k settlements |

## Delivery decision

No open P0/P1. Member point reversal is gated on MVP-15. Status: **done** (M3 gate).
