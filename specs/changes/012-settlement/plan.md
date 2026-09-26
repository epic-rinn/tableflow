# Change: 012-settlement — Cashier settlement

Status: done (M3 gate passed 2026-09-26 UTC). Date: 2026-09-27. Scope owner: Claude. Task: MVP-12. Verification: M3 gate (ADR-0004).

## Problem and behavior

Maps BIL-002, BIL-005, BIL-006 and BIL-008, reopening from BIL-007, ORD-007 and ADM-004. Cashiers:
- **begin** settlement at the observed bill version: freeze ordering, snapshot the totals, and move the visit to `settling`;
- **reopen** it with a reason (bumps the version);
- **confirm** receipt of the exact payable amount with method and verification note: creates the unique settlement and receipt reference, marks the visit paid, revokes dining access, and audits.

The table stays claimed until departure (SEA-004). Settlement is for non-members only until MVP-15 integrates loyalty atomically.

## Decisions

- **States:** visit states gain `settling`. Begin (`expected_version` = `bill_version`) locks the visit and refuses in these cases:
  - lines still `submitted` or not yet `served` → `409 UNRESOLVED_LINES`, listing the items;
  - a version mismatch → `409 BILL_VERSION_CONFLICT`, with the fresh bill in the body.
  - a visit with no chargeable line → `409 NOTHING_TO_SETTLE` (close it as empty instead).
  On success it stores a `bill_snapshots` row (totals, policy version, lines JSON) and bumps `bill_version`.
- **Serialisation:** order submission and line cancellation already require `open` and lock the same visit row, so a concurrent submission either commits first (and must be resolved before settlement) or fails because settling began (ORD-A4).
- **Confirm:**
  - staff roles: cashier or manager, checked from the authenticated session before an idempotent replay and re-validated in the transaction;
  - the amount must equal the snapshot total (`422 AMOUNT_MISMATCH`);
  - method is one of `cash`, `bank_transfer`, `card`, `other`; the verification note (1–500 characters) is required; the external reference (≤200) is optional;
  - `settlements.visit_id` is unique;
  - a concurrent or late confirmation of a paid visit gets `409 ALREADY_PAID` carrying the receipt reference (BIL-A3), while the same key replays the original;
  - receipt reference: `R-` plus 10 random base32 characters, unique;
  - guest sessions and uploaded material can never reach this route (BIL-A4).
- **Reopen:** settling → open with a reason; bumps `bill_version`, so an old confirmation version is rejected (BIL-A5).
- **Paid visits:** paid and departed visits never reopen.

## Verification map

| Requirement | Test | Status |
| --- | --- | --- |
| BIL-A1 stale bill version | TestBeginRejectsStaleBill | passed |
| Unresolved kitchen lines | TestBeginRequiresResolvedLines | passed |
| BIL-A3 two cashiers, response loss | TestConcurrentConfirmOneSettlement | passed |
| BIL-A4 guest credentials cannot settle | TestGuestCannotSettle | passed |
| BIL-A5 reopen invalidates old version | TestReopenInvalidatesConfirmation | passed |
| ORD-A4 settlement versus order submission | TestSettlementVersusOrderSubmission | passed |
| Exact amount, frozen ordering, access revoked, table retained, paid → depart → ready | TestConfirmEffects, admin cashier.spec.ts | passed |
| No zero-value settlement | TestBeginRequiresCharges | passed |
