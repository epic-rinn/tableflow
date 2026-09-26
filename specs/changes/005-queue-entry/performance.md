# Performance and DB/API review: M1 (005 queue entry, 006 call/seat, 007 lifecycle)

Date: 2026-09-26 (M1 milestone gate). Reviewer: Claude (self-review). Shared by [006](../006-call-seat/performance.md) and [007](../007-table-lifecycle/performance.md).

## Environment and fixture

MacBook arm64 (10 CPU, 24 GiB), Docker 29.1.3, `postgres:18.6` without limits, Go 1.27.1 API on the same host (pool 10), Python load client. This is **not** the canonical 2 vCPU/1 GiB profile.

The fixture `src/api/tests/performance/seating_seed.sql` matches the performance spec for this slice: one branch, 100 tables (70 occupied, 10 held, 10 cleaning, 10 available), 490 waiting and 10 called tickets, 70 open visits, 100,000 historical terminal tickets and 100,000 historical visits, `ANALYZE`d. The load run seeded 300 guest sessions and one host session.

## Query plans (warm cache)

| Statement (route) | Plan | Execution | Buffers |
| --- | --- | --- | --- |
| ticket_view (guest tracking; last waiting ticket) | PK + index-only count on partial `queue_tickets_waiting_group` | 0.171 ms | 70 |
| queue_board first page (host board) | partial `queue_tickets_active` in join order | 0.105 ms | 55 |
| queue_board `state=called` | same index, filter | 0.232 ms | 213 |
| counter_next (join) | PK upsert | 0.329 ms | 22 |
| tables_board (floor) | 100-row table scan + claims + ticket PK | 0.110 ms | 46 |
| older_compatible (fairness) | partial waiting index | 0.025 ms | 9 |
| visit_view | PK | 0.042 ms | 6 |
| visit_peek / claim_move (move) | PK / unique visit claim | 0.016 / 0.217 ms | 4 / 26 |

Historical tickets and visits are never scanned by polling routes: both boards and position counts use partial indexes on active states. Scans of the 100-row `dining_tables` are the planner's cheapest choice; no index is proposed. Raw plans are in [evidence/](evidence/) and the sibling packets' evidence folders.

## Concurrent polling workload ([evidence](evidence/polling-load.jsonl))

The workload follows the spec's shape: 300 guest trackers every 10 s ±20%, 20 staff clients each fetching the queue board (limit 100) and floor board every 3 s ±20%, and 10 host joins per second (new idempotency keys), for about 53 requests/s. It ran with 30 s warm-up and 120 s of measurement, shorter than the spec's 2 + 10 minutes.

| Route | Samples | Status | p50 / p95 / p99 ms |
| --- | --- | --- | --- |
| GET `/queue-tickets/{id}` (guest poll) | 3586 | 200 ×3586 | 2.42 / 3.72 / 4.80 |
| GET queue board | 799 | 200 ×799 | 3.01 / 4.34 / 5.50 |
| GET floor board | 799 | 200 ×799 | 2.11 / 3.26 / 5.34 |
| POST join (host, idempotent transaction) | 1180 | 201 ×1180 | 8.95 / 12.80 / 16.98 |

API CPU averaged 3.8% of one core, with peak RSS 56 MiB and no errors logged. The waiting list grew from 490 to 1968 during the run, and board latency stayed flat. All routes are well inside the proposed budgets (≤200 ms for polling and ≤500 ms for mutations at p95), but on a much larger machine than the canonical profile.

A first run was invalid because of a **fixture defect**: the seed created today's tickets without the matching `queue_counters` row. Every join then collided on a display number and got a sanitized 503. The seed was fixed and the run repeated; the first run is not used as evidence. See the counter-consistency note in the 005 review.

## Gaps

No pool-wait metrics are exported. Contention on the same table at production scale was covered by correctness tests, not measured. Browser (LCP/INP) metrics were not measured.
