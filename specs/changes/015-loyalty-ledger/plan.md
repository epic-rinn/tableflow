# Change: 015-loyalty-ledger — Atomic loyalty and member history

Status: done (M4 gate passed 2026-09-27). Date: 2026-09-27. Scope owner: Claude. Task: MVP-15. Verification: M4 gate (ADR-0004).

## Problem and behavior

Maps LOY-004–007 and complete member behaviour for BIL-006/007.

- **Payment:** payment of a claimed visit earns points and qualifying spend in the same transaction as the settlement.
- **Refund:** a full refund appends a compensating entry using the original amounts.
- **History:** members see balance, tier, progress and paginated history.

## Decisions

- **Eligible spend (LOY-004):**
  - `net = gross − discount`;
  - exclusive: `eligible = net`;
  - inclusive: `eligible = net − round_half_up(net × tax_bp / (10000 + tax_bp))`, which is food tax only and never tax on the service charge;
  - `points = floor(eligible / satang_per_point)` from the snapshot's policy;
  - every menu item counts as food in MVP.
- **Ledger:**
  - `loyalty_ledger` rows are `earn` or `reversal`, with signed points and qualifying delta and the policy version;
  - `UNIQUE (settlement_id, kind)`;
  - `settlements` gains `member_id`, `eligible_satang`, `points_earned` and `loyalty_policy_version`.
- **Transactions:**
  - confirm locks visit → member profile → capability → settlement insert → ledger → profile update;
  - refund locks visit → member profile → settlement → refund → reversal → profile update;
  - both then recalculate the tier from qualifying spend using the current policy thresholds.
- **Balance:** the balance never goes negative (no spending in MVP).
- **Member routes:**
  - `GET /members/me/loyalty` returns one profile per branch (tier, points, qualifying spend, next tier and threshold);
  - `GET /members/me/loyalty/entries?cursor&limit` is a keyset page, newest first.

## Verification map

| Requirement | Test | Status |
| --- | --- | --- |
| LOY-004 fixtures (exclusive, inclusive, discount) | TestEligibleSpendFixtures | passed |
| LOY-A2 concurrent paid visits for one member | TestConcurrentMemberSettlements | passed |
| LOY-A4 / BIL-A6 refund reverses once with original amounts | TestMemberRefundReversesOnce | passed |
| LOY-A3 tier threshold crossing | TestTierSnapshotAtBegin | passed |
| Ledger reconciles to balance; history pagination and privacy | TestMemberLoyaltyHistory | passed |
