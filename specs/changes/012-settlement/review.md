# Review: 012-settlement

Date: 2026-09-26 UTC (M3 milestone gate, ADR-0004). Reviewer: Claude — **self-review**. Scope: the `settling` state, `bill_snapshots` and `settlements`, begin/reopen/confirm, capability revocation, the admin cashier workspace, and seating/ordering guards for `settling`. Result: **pass**.

## Findings

| Severity | Location | Evidence | Impact | Resolution |
| --- | --- | --- | --- | --- |
| P2 | `internal/billing/settlement.go` BeginSettlement | Review: a visit whose lines were all rejected or cancelled could be settled for 0.00, producing a receipt | Misleading zero receipts; close-empty (SEA-004) is the intended path | Fixed: `409 NOTHING_TO_SETTLE`; TestBeginRequiresCharges. The UI already disabled begin without lines |
| P3 | `CashierWorkspace` confirm retry | The idempotency key is per visit and bill version. Editing the note after an ambiguous failure and retrying sends a different body under the same key | The server answers `IDEMPOTENCY_CONFLICT` (no double charge); the next retry shows `ALREADY_PAID` if the first attempt committed | Accepted: safe; message wording can improve in MVP-18 |
| P3 | Rotate-access on paid visits (pre-existing, MVP-07) | `RotateAccess` accepts `paid` visits | Could reissue read-only guest access after payment; ordering stays refused by state | Recorded for MVP-18 journey hardening |

Checked:
- **Serialisation:** begin, reopen and confirm lock the same visit row as order submission and cancellation. ORD-A4 was repeated 5×; exactly one of begin and order wins.
- **Stale versions:** a stale version returns the fresh bill (BIL-A1). Unserved lines block begin and are listed.
- **Frozen snapshot:** it survives policy changes, and ordering is refused while settling.
- **Reopen:** it needs a reason, bumps the version, and makes old confirmations fail (BIL-A5).
- **Confirm (BIL-A3):** requires the exact snapshot total (`AMOUNT_MISMATCH`). Two cashiers plus a response-loss retry produce one settlement, one audit event and the same receipt on replay; the loser gets `ALREADY_PAID` with the receipt reference.
- **Credentials (BIL-A4):** guest sessions never settle (origin guard/401), and host staff get 403. Roles are checked before any replay.
- **Payment effects:** payment revokes the QR and existing diner sessions. The table stays claimed until departure, then goes to cleaning and ready (browser journey). Paid bills cannot be begun or reopened.

## Verification

| Command / test | Result |
| --- | --- |
| `make verify` (final, 2026-09-26T18:23Z UTC; macOS arm64, Go 1.27.1, Node 24.21.0, pnpm 12.6.0, postgres:18.6, Chrome) | **passed**: 142 Go tests (race, PostgreSQL, none skipped), admin 11/11 and PWA 15/15 browser tests, artifact check (2783 files / 100 sentinels) |
| Earlier M3 gate runs | Run 1 (`api-test-db`): route-parity test lacked the billing provider; guest→staff settlement was refused with 403 by the origin guard, where the test expected only 401; the revoked-QR check used a helper that fails on exchange. Tests fixed; product behaviour was correct. Run 2 (`make verify`): passed. Run 3, after the review fixes: the begin-versus-order race fixture had no lines and now correctly got `NOTHING_TO_SETTLE`; fixture fixed and the test repeated 5 times. Run 4: passed |
| MVP-12 tests | TestBeginRequiresResolvedLines, TestBeginRejectsStaleBill (BIL-A1), TestSettlementVersusOrderSubmission (ORD-A4), TestReopenInvalidatesConfirmation (BIL-A5), TestConcurrentConfirmOneSettlement (BIL-A3), TestGuestCannotSettle (BIL-A4), TestConfirmEffects, TestBeginRequiresCharges; admin `cashier.spec.ts` (begin → cash change → confirm → depart → clean), `staff.spec.ts` (manager sees Cashier) |
| Load | [performance](performance.md): begin p95 17.9 ms, confirm 13.3 ms, reopen 11.4 ms at about 50 req/s |

## Delivery decision

No open P0/P1. Settlement is enabled for non-members only; loyalty awards join the confirm transaction in MVP-15. Status: **done** (M3 gate).
