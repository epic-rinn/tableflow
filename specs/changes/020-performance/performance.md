# Performance qualification (MVP-20)

Date: 2026-09-26T22:30Z–22:43Z UTC. Harness: `src/api/tests/performance/qualify.sh` + `qualify_load.py` + `qualify_sessions.sql` (reproducible; builds the fixture from the seed scripts). Raw evidence: [evidence/](evidence/).

## Environment ([environment.txt](evidence/environment.txt))

| Item | Value |
| --- | --- |
| Host | Apple M5, 10 CPUs, 24 GiB, macOS (Darwin 25.5.0); Docker 29.1.3, linux/arm64 VM |
| PostgreSQL | 18.6 in a container limited to **2 vCPU / 2 GiB**; `shared_buffers=512MB`, `effective_cache_size=1536MB`, `max_connections=100`, `log_lock_waits=on` |
| Go API | go1.27.1 static linux/arm64 binary in a container limited to **2 vCPU / 1 GiB**; `DB_MAX_CONNS=10` (default); `DB_POOL_STATS_INTERVAL=10s` |
| Load generator | Python 3.14 threads (381) on the host, outside both limits |
| Warm-up / measured | 120 s / 600 s |

This matches the canonical profile in [performance](../../quality/performance.md), except that the machine and network are local, not a staging region.

## Fixture ([fixture-counts.txt](evidence/fixture-counts.txt))

| Entity | Count |
| --- | --- |
| Tables | 100 |
| Waiting parties | 490 |
| Open visits | 70 |
| Historical visits | 100,000 |
| Menu items | 500 (1,000 option groups, 3,000 options) |
| Order lines | 1,000,840 |
| Members | 70,000 |
| Settlements | 100,000 |
| Loyalty ledger rows | 61,000 |
| Audit events | 250,000 |

Everything is `ANALYZE`d.

- **Active visits:** the profile asks for 200, but 100 tables allow at most 100 concurrent visits (one visit per table, SEA-002). The fixture uses 70, of which 35 are settle-ready member visits.
- **Waiting parties:** 490 rather than 500 after seeding.

## Workload

- **Foreground guests:** 150 queue trackers and 150 diners polling every 10 s ±20%. Diners read orders every poll, the bill every other poll, and one menu category (gzip) every 6th poll.
- **Staff:** 20 polling every 3 s: 10 host boards (queue plus tables), 5 kitchen boards, 5 cashier bill reads.
- **Mutations, about 10/s:**
  - 6/s assisted orders (random items, required options);
  - 1/s assisted queue joins;
  - about 3/s settlement cycles (member bill → begin → reopen), with 13 confirmations.
- **Other readers:** 50 members reading loyalty and history every 30 s; a manager reading the 31-day report and the audit log every 60 s.

Shared-table contention comes from 35 visits receiving assisted orders from 6 threads. Member contention comes from settlement cycles on claimed visits.

## Results ([load.jsonl](evidence/load.jsonl))

41,534 requests in 600 s (**69.2 req/s**), **0 unexpected errors** (no 5xx, no timeouts, no client exceptions). The 775 diner 401s are expected: payment during the run revoked those visits' dining sessions (BIL-006).

| Route | Samples | p50 / p95 / p99 ms | Avg bytes | Budget p95 |
| --- | --- | --- | --- | --- |
| GET queue ticket (tracker) | 9006 | 2.36 / 3.89 / 4.87 | 496 | 200 |
| GET visit orders (diner) | 8995 | 3.32 / 6.15 / 7.39 | 27,322 | 300 |
| GET visit bill (diner) | 4521 | 2.12 / 3.72 / 5.35 | 15,989 | 300 |
| GET menu category (gzip) | 1531 | 4.41 / 6.49 / 8.29 | 8,505 | 300 |
| GET queue board (host) | 1991 | 3.31 / 5.41 / 6.84 | 50,164 | 200 |
| GET tables (host) | 1991 | 2.19 / 3.77 / 5.19 | 19,665 | 200 |
| GET kitchen lines | 996 | 3.68 / 5.54 / 6.48 | 32,499 | 200 |
| GET visit bill (cashier) | 2775 | 3.25 / 5.14 / 6.40 | 8,044 | 300 |
| POST assisted order | 3568 | 5.28 / 7.64 / 9.32 | 723 | 500 |
| POST assisted queue join | 592 | 5.38 / 8.60 / 9.73 | 575 | 500 |
| POST settlement begin (member) | 1778 | 4.22 / 6.36 / 8.31 | 2,765 | 500 |
| POST settlement reopen | 1765 | 2.93 / 4.51 / 6.17 | 2,762 | 500 |
| POST settlement confirm (member) | 13 | 4.96 / 7.58 / max 7.37 | 3,111 | 500 |
| GET member loyalty | 997 | 2.42 / 4.21 / 5.77 | 264 | 300 |
| GET member loyalty entries | 997 | 1.85 / 3.32 / 4.15 | 837 | 300 |
| GET daily report (31 days) | 9 | 38.3 / 43.8 / 44.6 | 14,949 | 1000 |
| GET audit events | 9 | 1.86 / 2.45 / 2.52 | 17,825 | 1000 |

- **Budgets:** every route is inside its proposed budget. The largest responses (the host queue board at 50 KB, visit orders at 27 KB) are under the 100 KiB paginated-response guardrail.
- **Small samples:** the confirm and report p99 values are interpolated from few samples.
- **Tail latency:** the worst single requests were 45–55 ms. They are isolated (p99 ≤ 10 ms everywhere except the report), with no pool waits or lock waits at those moments.

## Resources, pool and locks

- **API container:** CPU averaged 7.3% of one core while busy and peaked at 11.2%, out of its 2-vCPU allowance (200%). Peak memory was 46 MiB of 1 GiB.
- **PostgreSQL container:** 7.8% average CPU while busy, 17.6% peak; 760 MiB of 2 GiB (shared buffers warm).
- **Pool ([pool-stats.log](evidence/pool-stats.log), 10 s samples):** up to 3,297 acquires per 10 s. At most 5 connections were open and 1 in use at a sampled instant. 5 acquires waited in total, with a maximum average acquire of 0.003 ms: no pool saturation.
- **Locks ([database.json](evidence/database.json)):**
  - 699 one-second samples of `pg_stat_activity` found **0** lock waiters (at most 3 active backends, at most 7 connections);
  - 0 deadlocks and 0 `log_lock_waits` entries;
  - 0 temp files, 99.33% buffer cache hit, 168,502 commits.

## Plans

- **Read plans:** recorded per milestone; representative-history plans are in the M1–M5 evidence folders.
- **Write plans:** hot write statements in rolled-back transactions ([write-plans.txt](evidence/write-plans.txt)).

  | Statement | Warm time | Access |
  | --- | --- | --- |
  | visit lock (`FOR UPDATE`) | 0.08 ms | primary key |
  | visit state and bill-version bump | 0.29 ms | primary key |
  | member profile update | 0.12 ms | primary key |
  | bill-version bump | 0.13 ms | primary key |

## Optimizations

None were needed: no budget was exceeded, so nothing was changed for speed. The only code added is pool-statistics logging (observability), covered by `TestPoolStatsInterval`.

## Gaps

- **Location:** local machine and Docker VM, not a staging region; the load generator shares the host.
- **Active visits:** capped by the table count, as noted above.
- **Browser experience:** mobile LCP, INP and CLS were not measured in the lab; client bundle budgets are recorded in the [M5 performance](../016-reporting/performance.md) review.
- **Scale:** a single API replica, as the architecture specifies.
