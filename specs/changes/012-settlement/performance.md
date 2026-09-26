# Performance and DB/API review: M3 (011 bill calculation, 012 settlement, 013 refunds)

Date: 2026-09-26 UTC (M3 milestone gate). Reviewer: Claude (self-review). Shared by [011](../011-bill-calculation/review.md) and [013](../013-refunds/review.md).

## Environment and fixture

Same host as M1/M2: MacBook arm64, `postgres:18.6` without resource limits, Go 1.27.1, pool 10. This is **not** the canonical 2 vCPU/1 GiB profile.

`src/api/tests/performance/billing_seed.sql` sits on top of the M2 fixture (100,070 visits, 1,002,316 order lines) and adds:
- 20 charge-policy versions;
- one frozen snapshot plus settlement for each of the 100,000 departed visits, with unique receipt references;
- 1,000 refunds;
- all active lines of 35 of the 70 open visits set to served, so they can settle.

Everything is `ANALYZE`d.

## Query plans ([011](../011-bill-calculation/evidence/), [012](evidence/), [013](../013-refunds/evidence/))

| Statement | Plan | Execution | Buffers |
| --- | --- | --- | --- |
| bill_lines (one visit, 1M lines in table) | `order_lines_by_visit` | 0.095 ms | 45 |
| policy_current (20 versions) | `charge_policies_pkey` backward, limit 1 | 0.018 ms | 2 |
| visit_get / visit_lock (`FOR UPDATE OF v`) | `visits_pkey` + 100-row table scan | 0.045 / 0.055 ms | 6 / 8 |
| snapshot_get (100k snapshots) | `bill_snapshots_visit_id_bill_version_key` | 0.053 ms | 2 |
| settlement_for_visit | `settlements_visit_id_key` | 0.026 ms | 4 |
| receipt_get (settlement + snapshot + visit + table + staff + refund) | primary/unique keys; 200-row staff scan | 0.084 ms | 21 |
| receipts_page (first page of 100k) | `settlements_by_branch` keyset | 0.123 ms | 242 |
| receipts_page by reference | `settlements_receipt_reference_key` | 0.065 ms | 22 |
| refund_get | `refunds_settlement_id_key` | 0.019 ms | 2 |

The only sequential scans are over `dining_tables` (100 rows) and `staff_accounts` (about 200 rows), which is the planner's cheapest choice at this size. No index is proposed.

## Load run ([evidence/billing-load.jsonl](evidence/billing-load.jsonl))

Workload, with 30 s warm-up and 120 s measured:
- 300 diner phones polling their bill every 10 s ±20%;
- 10 cashiers polling random bills every 3 s;
- 5 receipt-list readers (page plus one detail) every 5 s;
- 5 cashier threads running GET bill → begin → reopen cycles on the 35 settle-ready visits, confirming each visit once at a random time.

That is about 50 requests/s.

| Route | Samples | Status | p50 / p95 / p99 ms |
| --- | --- | --- | --- |
| GET bill (diner) | 3589 | 200 ×3402, 401 ×187 | 4.32 / 6.39 / 7.83 |
| GET bill (cashier) | 984 | 200 ×984 | 4.55 / 7.72 / 9.99 |
| POST settlement/begin | 569 | 200 ×569 | 11.96 / 17.94 / 22.17 |
| POST settlement/reopen | 534 | 200 ×534 | 7.60 / 11.36 / 16.44 |
| POST settlement/confirm | 35 | 201 ×35 | 10.26 / 13.30 / max 13.56 |
| GET settlements (page of 25) | 120 | 200 ×120 | 4.48 / 6.16 / 12.47 |
| GET settlement (receipt) | 120 | 200 ×120 | 1.87 / 3.56 / 5.19 |

The 187 diner 401s are expected, not errors: those diners' visits were paid during the run, and payment revokes dining sessions (BIL-006). Every settle-ready visit was confirmed exactly once. The confirm p99 is interpolated from only 35 samples. API CPU averaged 5.1% of one core, peak RSS was 57 MiB, and no errors were logged. All routes are inside the proposed latency budgets.

## Request cost

- **Bill read:** authentication plus 3 statements (visit, lines, policy) live, or visit plus snapshot plus settlement when frozen. `TestBillStatementsConstant` proves the count is the same for 1 and 30 lines.
- **Begin:** a single transaction of about 8 statements, locking the visit row only. **Confirm:** about 9 statements, including capability and session revocation.
- **Idempotency:** replays are served before any domain statement, after the role check.
- **Bounds:** receipt pages are capped at 100.
- **Unbounded bill:** a visit's bill is not paginated. Orders are capped at 50 lines each, but the number of orders per visit is not capped (see review P3).

## Gaps

- The canonical resource-limited profile was not used.
- Pool-wait metrics are not exported.
- Real operator menus, rates and bill sizes have not been measured.
