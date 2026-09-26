# Performance and DB/API review: 001-foundation

Date: 2026-09-26. Reviewer: Claude (self-review; no independent reviewer ran). Scope: infrastructure data paths only. Domain-load budgets in [performance](../../quality/performance.md) are **not applicable** and no restaurant-load qualification is claimed.

## Changed routes and data paths

| Route / path | Auth | DB work | Bounds | Cache |
| --- | --- | --- | --- | --- |
| `GET /api/v1/health/live` | Public | None | Fixed 16-byte body | `no-store` |
| `GET /api/v1/health/ready` | Public (also reachable via both frontend proxies) | One pgx ping (`-- ping` statement) per in-flight probe; concurrent callers share it (`singleflight`) | 1 s probe timeout, detached from caller cancellation; ≤1 pool connection used by readiness at any time | `no-store` |
| Unknown paths / wrong method | Public | None | JSON 404/405 envelope | `no-store` |
| `cmd/migrate` | Owner role via `MIGRATION_DATABASE_URL` | Goose version table + migration SQL | Release step only | n/a |

No domain SQL, transactions, locks, pagination or frontend data fetching exist yet. Frontend pages are static prerendered shells; the only browser→API traffic is the smoke test.

## Pool and timeouts (static review)

`pgxpool` max 10 connections (configurable 1–100), no minimum, idle 5 min, lifetime 1 h, health-check 1 min. Session parameters: `statement_timeout` 5 s, `lock_timeout` 2 s, `idle_in_transaction_session_timeout` 10 s, `timezone=UTC`, `application_name=tableflow-api` (confirmed in `pg_stat_activity` with user `tableflow_app`). HTTP server timeouts: read-header 5 s, read 10 s, write 15 s, idle 60 s, headers ≤64 KiB. Shutdown: 10 s drain, then pool close bounded at 3 s. Pool size versus PostgreSQL `max_connections` (default 100) leaves headroom for one replica; multi-replica sizing is deferred to MVP-21.

## Measurements

Environment: MacBook arm64 (10 CPU, 24 GiB), Docker 29.1.3, `postgres:18.6` without container resource limits, API built with Go 1.27.1 on the same host, sequential Python `urllib` client after 20 warmup requests. This is **not** the canonical 2 vCPU/1 GiB profile and not a load test; numbers only establish that health routes are cheap and bounded.

| Condition | Route | Samples | Status | p50 / p95 / p99 ms | Bytes |
| --- | --- | --- | --- | --- | --- |
| PostgreSQL up | live | 1000 | 200 ×1000 | 0.10 / 0.16 / 0.19 | 16 |
| PostgreSQL up | ready | 1000 | 200 ×1000 | 0.28 / 1.33 / 1.75 | 46 |
| PostgreSQL stopped | ready | 200 | 503 ×200 | 0.20 / 0.25 / 0.26 | 137 |
| PostgreSQL stopped | live | 200 | 200 ×200 | 0.10 / 0.15 / 0.18 | 16 |
| PostgreSQL paused (no replies) | ready | 3 sequential + 20 parallel | 503 all | ≈1003 sequential; 993–994 parallel | 137 |

Raw results: [health-latency.jsonl](evidence/health-latency.jsonl), [dependency-and-shutdown.txt](evidence/dependency-and-shutdown.txt). Unit tests independently establish the timeout bound (`TestReadinessTimeoutIsBounded`, 200 ms → 201 ms) and probe coalescing (`TestConcurrentReadinessSharesOneProbe`: 50 callers, 1 probe, peak concurrency 1).

## Findings from this pass

- Readiness is reachable by anyone who can reach either frontend origin. It reveals only ready/not-ready; each failure is logged at WARN, so a request flood during an outage amplifies logs (bounded to sequential probes by coalescing). Recorded as P3 in [review](review.md); revisit with monitoring/deployment routing in MVP-21.
- No EXPLAIN plans apply: the only statement is the pgx ping. No indexes proposed.
- Measurement gaps: no concurrent HTTP load, no resource-limited profile, no pool-wait metrics (none are exported yet). These gaps do not block MVP-01 because it has no domain paths; they must be addressed with the first domain endpoints (MVP-02 onward) and the load runner selection.
