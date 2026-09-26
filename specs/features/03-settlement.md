# Bills and cashier settlement

Status: specified; implemented and verified in M3 for non-members (MVP-11–13, gate passed 2026-09-26 UTC). Member discount, points award and reversal wait for M4 (MVP-14/15). Actors: guest (read/request), cashier, manager.

## Requirements

- **BIL-001** Each visit has one bill. Dining QR identifies that visit for bill retrieval; it cannot authorize settlement. Only Go calculates charges from order snapshots and the active policy version.
- **BIL-002** Cashier begins settlement with the bill version observed. The service locks the visit, checks no submitted/pending-acceptance lines remain and every chargeable line is served, then snapshots totals and enters `settling`. Otherwise return a conflict describing unresolved items. Freeze ordering and financial edits.
- **BIL-003** Before finalization show subtotal, eligible discount, service charge, tax, rounding, and payable total. Use integer arithmetic and one explicit rounding rule: round half up to the nearest satang per aggregate charge, not per item. All products in MVP use the same charge/tax configuration; mixed tax categories are deferred.
- **BIL-004** Proposed calculation: gross line subtotal minus the single eligible tier discount gives net subtotal; service charge is calculated on net subtotal. For tax-exclusive pricing, tax is computed on net subtotal plus service charge and added. For tax-inclusive pricing, tax is extracted from that same amount for display and is not added again. Policy versions specify tax mode/rates, service rate, and eligible items; require operator validation before launch. Do not hard-code statutory rates.
- **BIL-005** Cashier confirms receipt of the exact payable amount with method (`cash`, `bank_transfer`, `card`, `other`), optional external reference, and verification note. Cash tender/change may be displayed, but recorded paid amount equals bill total. Cashier must verify external receipts using the restaurant's trusted source; uploaded slips are not automatic confirmation.
- **BIL-006** In one transaction create the unique settlement, mark the bill/visit paid, revoke dining ordering access, award eligible loyalty once, and append audit events. Retrying or concurrent confirmation cannot duplicate any effect. If the transaction fails, leave the bill unsettled and let the cashier reconcile the received money before retrying.
- **BIL-007** Cashier can reopen a settling bill before payment, invalidating the prior snapshot and incrementing version. Paid bills cannot reopen. A manager can record a full refund with reason and externally completed refund reference; atomically reverse points/tier credit once and retain the original settlement. Partial refunds and automated money movement are excluded.
- **BIL-008** Paid bill confirmation yields an opaque receipt reference. No payment event alone releases the occupied table; follow SEA-004. Staff can retrieve historical receipts without reactivating guest ordering access.

## Acceptance scenarios

| ID | Given / When / Then |
| --- | --- |
| BIL-A1 | Given a displayed bill version, when another financial change commits before begin-settlement, then settlement receives a conflict and fresh bill (BIL-002). |
| BIL-A2 | Given tax-inclusive and exclusive fixtures with fractional-satang percentages, then totals match approved examples exactly with no floating-point drift (BIL-003/004). |
| BIL-A3 | Given two cashiers confirming one settling visit, then one settlement and one points award exist; the loser receives the already-paid outcome (BIL-005/006). |
| BIL-A4 | Given a forged slip or guest-only credential, when payment confirmation is requested, then it cannot close the bill (BIL-005). |
| BIL-A5 | Given a settling bill, when reopened and reordered, then an old confirmation version is rejected (BIL-007). |
| BIL-A6 | Given a paid member bill, when its full refund is recorded twice, then exactly one reversal exists and the visit stays closed (BIL-007). |

Daily manager summaries separate sales, refunds, payment methods, and service day; totals must reconcile to recorded settlements, not current mutable menu prices.
