# Performance and DB/API review: M2 (008 menu, 009 ordering, 010 kitchen)

Date: 2026-09-26 (M2 milestone gate). Reviewer: Claude (self-review). Shared by [008](../008-menu/performance.md) and [010](../010-kitchen/performance.md).

## Environment and fixture

Same host as M1 (MacBook arm64, `postgres:18.6` without limits, Go 1.27.1, pool 10); **not** the canonical 2 vCPU/1 GiB profile. `tests/performance/ordering_seed.sql` on top of the M1 fixture adds:
- a 500-item menu (10 categories × 50 items; 1,000 option groups and 3,000 options, all bilingual);
- 100,000 historical orders with **1,000,000** served or cancelled lines;
- 840 active kitchen lines across the 70 open visits;
- 50,000 resolved and 30 open assistance requests.

Everything is `ANALYZE`d. The spec's 200 active visits cannot exist with 100 tables and one visit per table, so the fixture uses 70.

## Query plans ([008](../008-menu/evidence/), [009](evidence/), [010](../010-kitchen/evidence/))

| Statement | Plan | Execution | Buffers |
| --- | --- | --- | --- |
| menu items (500) / groups + options (3,000) | small-table scans | 0.17 / 1.65 ms | 16 / 73 |
| items_for_order (20 items, `FOR SHARE`) | primary key | 0.049 ms | 23 |
| groups_for_items (20 items) | bitmap on `option_groups_by_item`, options hash | 0.33 ms | 54 |
| orders_page / lines_for_orders / visit_totals | `orders_by_visit` / `order_lines_by_order` / `order_lines_by_visit` | 0.021 / 0.048 / 0.043 ms | 6 / 27 / 15 |
| kitchen_board (101 rows, 1M historical lines) | partial `order_lines_kitchen` + visit and table primary keys | 0.200 ms | 554 |
| assistance_board (50k resolved) | partial `assistance_board` | 0.095 ms | 186 |

No polling query touches historical lines or requests. The only sequential scans are over the 3,000-row options table in the menu read, which is the planner's cheapest choice at this size; no index is proposed.

## Load run ([evidence/ordering-load.jsonl](evidence/ordering-load.jsonl))

Workload: 300 diner phones polling their visit's orders every 10 s ±20%, 20 staff polling the kitchen board every 3 s, 10 assisted orders per second with new keys, and 2 clients fetching the full menu every 5 s. That is about 47 requests/s, with 30 s warm-up and 120 s measured.

| Route | Samples | Status | p50 / p95 / p99 ms |
| --- | --- | --- | --- |
| GET visit orders (diner poll) | 3609 | 200 ×3609 | 5.03 / 8.35 / 13.54 |
| GET kitchen lines | 795 | 200 ×795 | 3.73 / 6.40 / 18.76 |
| POST order (assisted, idempotent) | 1180 | 201 ×1180 | 12.70 / 19.59 / 45.18 |
| GET full menu (500 items) | 48 | 200 ×48 | 17.3 / 34.9 / 38.9 |

API CPU averaged 6.1% of one core, peak RSS was 56 MiB, and no errors were logged. All routes are inside the proposed latency budgets.

## Menu response size ([008 evidence](../008-menu/evidence/menu-size.jsonl))

| Response | Bytes | p95 |
| --- | --- | --- |
| Full menu, uncompressed | **628,133 (613 KiB)** | 8.7 ms |
| Full menu, gzip (menu route only) | 81,599 | 9.5 ms |
| One category (50 items), uncompressed | 63,896 | 3.5 ms |
| One category, gzip | 8,700 | 3.3 ms |

The dense full menu exceeds the proposed **500 KiB uncompressed** budget. As the HTTP contract prescribes for menus over the bound, the API now supports a category split (`?category_id=`), measured at 62 KiB per category. The PWA still loads the full menu; see the P2 in the [MVP-08 review](../008-menu/review.md). Order-read statement counts were verified by test (a pgx tracer): the menu read uses 5 statements and the order read 5, independent of size.

## Gaps

Pool-wait metrics are not exported. The operator's real menu has not been measured, and the canonical resource-limited profile was not used.
