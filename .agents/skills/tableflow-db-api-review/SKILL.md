---
name: tableflow-db-api-review
description: Analyze TableFlow PostgreSQL queries, schema/migrations, Go endpoints, and frontend request patterns for correctness and measured performance. Run for changed data paths after implementation or explicit database/API review requests.
---

# Review database and API performance

Read [data model](../../../specs/architecture/data-model.md), relevant [HTTP routes](../../../specs/api/http.md), and [performance budgets](../../../specs/quality/performance.md). The budgets are proposed targets, not proof the system meets them.

## Trace the complete path

For each affected endpoint record authorization, input bounds, handler/use case, query files, transaction/lock scope, statement count, result cardinality, response size, polling frequency, and cache policy. Include authentication queries in measured totals even where a domain-query budget excludes them.

Check parameterized SQL, explicit selected columns, predicate/index ordering, branch scoping, stable cursor pagination, batched relationships, affected-row checks, and row/connection cleanup. Flag N+1 and unbounded reads. Verify constraints protect claims, idempotent effects, and financial uniqueness. Check lock ordering and transaction boundaries rather than assuming Go's race detector covers database races.

## Measure on disposable representative data

Use the fixture/workload in the performance spec or explain a representative feature-specific substitute. Record PostgreSQL version, hardware/resources, indexes/statistics, parameter distribution, and warmup. Capture `EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON)` for hot queries and endpoint latency under expected polling/concurrency. Include rare/large values, empty results, and common skewed values.

EXPLAIN ANALYZE executes SQL. Prefer read-only transactions for SELECT. Use a disposable database for write plans; rollback does not neutralize every side effect. Do not run load tests or write plans on production without explicit authorization. If no service/database is runnable, report static analysis and missing measurements; do not invent timings or count it as a measured pass.

Interpret actual versus estimated rows, loops, buffers, spills, lock waits, and pool acquisition. A sequential scan can be correct for a small table. Index proposals must specify predicate/order benefit and write/storage cost. Avoid latency claims based on plan cost alone.

## Check request and deployment cost

Review transaction duration, cancellation/timeouts, connection limits across replicas, retry amplification, body/page/date bounds, JSON size, server/client waterfalls, private cache leakage, and hidden-tab polling. Compare before/after under the same workload where a baseline exists. Include expected 409/429 separately from unexpected errors.

## Evidence and outcome

Write delivery evidence to `specs/changes/<id>/performance.md` with sanitized raw artifacts in `evidence/`; reference the main review report. Include per-route/query findings, actual commands/results, dataset, p50/p95/p99, error rate, query counts, concurrency invariants, and measurement gaps as applicable.

Classify findings using the code-review severity rules. Correctness/security takes precedence over speed. Do not add speculative indexes/caches or weaken isolation to hit a target. For delivery work, fix authorized blocking issues, rerun the affected workload, and review the final change. For review-only requests, report findings without modifying application code.
