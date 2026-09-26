# Optional loyalty

Status: specified; implemented and verified in M4 (MVP-14/15, gate passed 2026-09-27). Version 0 of the loyalty policy uses the documented pilot defaults until the operator confirms the economics. Actors: authenticated member, cashier, manager.

## Requirements

- **LOY-001** One member can claim a visit before settlement by using both their member session and that visit's guest capability. Shared dining QR access alone cannot read a member's identity/balance. A claim response exposes only claim status to other diners.
- **LOY-002** A different member cannot replace the claim silently. Staff may detach a claim while the visit is open with a reason; the replacement member must authenticate and claim it. Settling/paid claims are immutable.
- **LOY-003** Snapshot the member's current tier benefit at begin-settlement, after locking the member account. Apply at most one tier discount; no stacking. Show it before payment. Tiers earned from the current bill affect future bills only.
- **LOY-004** Eligible earning/tier amount is the post-discount food subtotal excluding tax/service charges. For tax-inclusive food prices, subtract the included food tax, rounded half up at the aggregate food-subtotal level; do not subtract service-charge tax from food spend. For tax-exclusive prices use the net food subtotal directly. Compute points as floor(eligible satang / configured satang per point). Store the applied policy version and eligible amount with the settlement.
- **LOY-005** Payment appends one earning ledger entry, increments qualifying-spend credit, and updates the balance/tier in the same transaction. A unique settlement/event constraint prevents retry duplication. Concurrent visits for one member serialize their balance updates.
- **LOY-006** Full refund appends a compensating entry for the original earned points and qualifying credit, then recalculates the tier. Never edit/delete the original ledger entry. Refund reversals use the original policy/snapshot, not today's rates.
- **LOY-007** The member page shows their balance, tier, progress threshold, and paginated history. Membership remains optional. Guests cannot retrospectively claim settled bills in MVP.

MVP awards points and grants discounts through tiers. Point spending, point expiry, coupons, and marketing messages are intentionally deferred; adding them requires a new spec for redemption authorization and reservation/concurrency behavior. Pilot defaults are in [MVP scope](../product/mvp.md).

## Acceptance scenarios

| ID | Given / When / Then |
| --- | --- |
| LOY-A1 | Given a shared QR, when an unauthenticated diner tries to claim points or read member data, then access is denied (LOY-001). |
| LOY-A2 | Given two simultaneous paid visits for one member, then both earnings appear once and the final balance equals the ledger sum (LOY-005). |
| LOY-A3 | Given a bill crossing a tier threshold, then its discount uses the previous tier and the next eligible visit can use the new tier (LOY-003). |
| LOY-A4 | Given changed loyalty rates, when an earlier bill is refunded, then its original award is reversed exactly once (LOY-006). |
