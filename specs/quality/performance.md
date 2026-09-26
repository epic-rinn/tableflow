# Performance targets and evidence

Status: proposed engineering budgets, not measured results or contractual SLAs. Correctness and access control remain mandatory. Optimize based on evidence rather than chasing minimum query time at any cost.

## Reproducible baseline

Start with a dedicated local/staging profile: Go API limited to 2 vCPU/1 GiB, PostgreSQL 18 at 2 vCPU/2 GiB, same host/network region. Record actual hardware, container limits, versions, connection counts, dataset seed, and warm/cold state. If unavailable, record the actual environment and do not compare its timings as equivalent.

Synthetic fixture: one branch, 100 tables, 500 waiting parties, 200 active visits, 500 menu items with options, 100,000 historical visits, 1,000,000 historical order lines, 50,000 members. Most history is terminal; include skew toward popular items/members and separate cross-branch isolation fixtures. Run `ANALYZE` after seeding.

Workload: 300 foreground queue clients at 10-second polls, 20 staff clients at 3-second polls, plus 10 mutations/second, with realistic think times and shared-table/member contention. Poll rates aggregate to about 37 reads/second before additional browsing. Warm up 2 minutes and measure 10 minutes; report sample counts, p50/p95/p99, errors, CPU, pool waits, and DB utilization. Do not omit polling from load tests.

## Initial budgets

| Path | Target p95 at the Go HTTP boundary | Database/response guardrail |
| --- | --- | --- |
| Queue status, table board, kitchen page | ≤200 ms | ≤3 domain read statements per request, excluding fixed auth lookup; no query per row |
| Menu / visit orders / bill | ≤300 ms | ≤4 domain reads; menu ≤500 KiB uncompressed, paginated operational response ≤100 KiB |
| Order / seating / settlement mutation | ≤500 ms | Short bounded transaction; statement count documented, independent of unbounded list growth |
| Daily report (≤31 days) | ≤1 second | Indexed date/branch filter; historical work outside polling routes |
| Hot SQL query | Normally ≤50 ms actual execution | Review outliers, scanned/returned rows, spills, lock waits, and buffers |

Report unexpected 5xx/timeouts separately from intentional conflicts and validation errors; target <0.5% unexpected failures in the baseline. No tolerated double charges/seats/points. These budgets include pool wait inside the API but exclude browser network/rendering; measure browser experience separately. Proposed mobile goals: p75 LCP ≤2.5 s, INP ≤200 ms, CLS ≤0.1 under a documented device/network profile; field and lab data are not interchangeable.

Avoid mechanical query-count gaming: a single unbounded join can be worse than several bounded reads. If a budget needs changing, record measured reason/tradeoff in the change plan, not a silent exception.

## SQL review procedure

For each new/changed hot query, record its file/name, calling endpoint, parameter distribution, table cardinalities, expected rows, indexes, round trips, and transaction/lock scope. Execute `EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON)` on representative parameters in a disposable database, save the plan and interpretation, and compare baseline where applicable.

`ANALYZE` executes statements. Use read-only transactions for SELECT plans when possible; mutating plans require a disposable database even if rolled back (sequences and other side effects may remain). Do not assume `EXPLAIN` without ANALYZE establishes actual speed. Do not reject sequential scans on small tables automatically.

Inspect estimated versus actual rows, loops/N+1, sort/hash spills, buffers, filter selectivity, and parameter skew. Run endpoint latency and contention tests as well; a fast standalone plan does not capture pool waits, locks, serialization, or JSON size.

## Endpoint and frontend review

Require bounded pagination/body/time range, cancellation/timeouts, no database work in loops over unbounded results, short transactions, explicit auth, and response-size evidence. Check indexes match branch/state/order predicates. Review pool capacity across replicas and reserve database connections for operations.

Check Next.js fetch waterfalls, duplicated server/client reads, cache isolation, hydration size, polling amplification, abort cleanup, image sizes, and unnecessary client JavaScript. Start with correctness-preserving SQL/request fixes; introduce caches only with invalidation/ownership rules and measured benefit.

Store results in `specs/changes/<id>/performance.md` and sanitized raw plans/results under that change's `evidence/`. A static review without a runnable service/database is **not measured** and cannot satisfy a runtime performance gate.
