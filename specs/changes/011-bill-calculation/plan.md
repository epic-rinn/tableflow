# Change: 011-bill-calculation — Bill calculation and policies

Status: implemented-unverified (M3 gate pending). Date: 2026-09-27. Scope owner: Claude. Task: MVP-11. Verification: M3 gate ([ADR-0004](../../decisions/0004-milestone-verification.md)).

## Problem and behavior

Maps BIL-001, BIL-003, BIL-004 and the charge-policy subset of OPS-001/ADM-005 ([settlement](../../features/03-settlement.md)).
- **Charge policy:** managers edit a versioned policy covering tax mode (`exclusive` or `inclusive`), tax rate and service-charge rate, both in basis points.
- **Bill:** Go computes the bill from the visit's order-line snapshots and the current policy; clients never send totals. Guests of the visit and staff read an itemised bill.

## Decisions (reversible unless noted)

- **Policy storage:** `charge_policies` rows are append-only versions per branch; the latest version is current. A branch with no policy uses version 0 (exclusive, 0% tax, 0% service), labelled *unconfigured*. Nothing claims statutory correctness or operator approval; the admin editor says rates must be confirmed with the operator before live use (BIL-004).
- **Calculation**, a pure Go function over integer satang, with each aggregate charge rounded half up exactly once (BIL-003):
  - `gross = Σ unit × qty` over chargeable lines (rejected and cancelled lines excluded);
  - `discount = round(gross × discount_bp / 10000)`, with discount 0 until the loyalty tasks MVP-14/15 supply a tier (canonical interface: `Calculate(lines, policy, discountBP)`);
  - `net = gross − discount`;
  - `service = round(net × service_bp / 10000)`;
  - `base = net + service`;
  - exclusive: `tax = round(base × tax_bp / 10000)`, `total = base + tax`;
  - inclusive: `tax = round(base × tax_bp / (10000 + tax_bp))` (shown, not added), `total = base`.
- **Bounds:** rates 0–10000 basis points; the arithmetic stays well inside int64.
- **Routes:**
  - `GET/PUT /branches/{id}/charge-policy` (staff read, manager write with `expected_version`), a split of the combined configuration route recorded in the HTTP contract;
  - `GET /visits/{id}/bill` for the visit's guests or cashier, manager and host staff;
  - `POST /bills/resolve {dining_token}` for cashiers, with the token in the body only.
- **Frozen totals:** while `settling` or `paid`, the bill shows the frozen snapshot from MVP-12, not a recomputation, so later policy changes never alter historical bills.

## Verification map

| Requirement | Test | Status |
| --- | --- | --- |
| BIL-A2 exclusive/inclusive, fractional rounding (hand-computed fixtures) | TestCalculateFixtures | written, not run |
| Rejected/cancelled lines and later menu price changes | TestBillUsesSnapshotsAndChargeableLines | written, not run |
| Policy versions, bounds, roles | TestChargePolicyVersions | written, not run |
| Bill access (guest own visit, staff, resolve by token) | TestBillAccess | written, not run |
