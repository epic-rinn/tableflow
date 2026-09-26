# Performance harness location

The executable harness will be added under `src/api/tests/performance/` with the first implemented data path. Use synthetic PostgreSQL fixtures and a pinned HTTP load runner selected in M0; prefer k6 for reproducible request mixes. Do not add unused test dependencies during this documentation setup.

Follow [performance budgets](../../specs/quality/performance.md). Keep executable seed/load scripts in the API test directory, module integration/concurrency tests near Go code, and per-change measurements/plans under that change's `evidence/` folder outside `src/`.

Every run records environment, fixture cardinality, warmup/duration, polling clients, offered load, latency distribution, unexpected errors, query plans, and pool/lock waits. A fast empty database is not representative evidence.
