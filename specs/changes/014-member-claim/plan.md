# Change: 014-member-claim — Member visit claim and tier snapshot

Status: in-progress. Date: 2026-09-27. Scope owner: Claude. Task: MVP-14. Verification: M4 gate (ADR-0004), together with MVP-15.

## Problem and behavior

Maps LOY-001–003 and the loyalty-policy subset of OPS-001/ADM-005 ([loyalty](../../features/04-loyalty.md)).

- **Claim:** a signed-in member at the table claims the visit with both their member session and the visit's guest session. Other diners only learn that the visit is claimed.
- **Detach:** staff may detach a claim with a reason while the visit is open. Settling and paid claims are immutable.
- **Tier snapshot:** begin-settlement locks the member profile and snapshots the tier discount (one discount, no stacking).
- **Policy:** managers edit a versioned loyalty policy.

## Decisions (reversible)

- **Policy:** `loyalty_policies` stores append-only versions per branch: `satang_per_point`, Silver/Gold thresholds (cumulative qualifying satang) and discounts (basis points); the Base tier has no discount.
  - Version 0 uses the pilot defaults in [MVP scope](../../product/mvp.md): 10,000 satang per point, Silver 500,000 → 300 bp, Gold 1,500,000 → 500 bp.
  - It is labelled "pilot defaults, not confirmed", like the charge policy.
  - Edits use the `(branch, version)` key, with no branch lock.
- **Profile:** `member_profiles (branch, member)` holds `points_balance`, `qualifying_spend_satang`, `tier` and `version`. It is created on first claim, and lock order is visits → member profiles → child rows.
- **Claim storage:** the claim lives on the visit (`member_id`, `member_claimed_at`), and claim and detach bump `bill_version`. Rules:
  - the same member claiming again is idempotent (200);
  - another member gets `409 ALREADY_CLAIMED`;
  - a non-open visit gets `409 VISIT_STATE_CONFLICT`.
  The member session is re-validated in the transaction, and the guest session after the visit lock.
- **Privacy:**
  - `GET /visits/{id}/member-claim` needs the member and guest sessions and returns `{claimed, mine}`; tier and discount are included only when `mine`.
  - The bill's `member_claim` is `{claimed}` for guests. Cashiers and managers also get a masked email and the tier.
  - The guest bill labels the discount "Member discount" without naming the tier.
- **Discount:**
  - Open bills preview the current tier discount.
  - Begin-settlement locks the profile and stores `member_id`, `tier`, `discount_bp`, `loyalty_policy_version` and `satang_per_point` in the snapshot. Later tier changes never alter it (LOY-A3).
- **Routes:**
  - `POST /visits/{id}/member-claim {expected_version}` (member + guest, PWA origin);
  - `POST /visits/{id}/member-detach {expected_version, reason}` (cashier/manager);
  - `GET /visits/{id}/member-claim`;
  - `GET/PUT /branches/{id}/loyalty-policy` (staff read, manager write).

## Verification map

| Requirement | Test | Status |
| --- | --- | --- |
| LOY-A1 shared QR cannot claim or read member data | TestClaimNeedsBothSessions | planned |
| No silent replacement; detach with reason; immutable once settling | TestClaimConflictsAndDetach | planned |
| LOY-A3 threshold crossing uses previous tier | TestTierSnapshotAtBegin (in 015 tests) | planned |
| Policy versions and bounds | TestLoyaltyPolicyVersions | planned |
