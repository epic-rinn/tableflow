# Performance harness location

The executable harness will be added under `src/api/tests/performance/` with the first implemented data path. Use synthetic PostgreSQL fixtures and a pinned HTTP load runner; prefer k6 for reproducible request mixes. MVP-01 only measured health routes with a sequential script and did not select or pin the load runner; select it with the first domain data path. Do not add unused test dependencies during this documentation setup.

Follow [performance budgets](../../specs/quality/performance.md). Keep executable seed/load scripts in the API test directory, module integration/concurrency tests near Go code, and per-change measurements/plans under that change's `evidence/` folder outside `src/`.

Every run records environment, fixture cardinality, warmup/duration, polling clients, offered load, latency distribution, unexpected errors, query plans, and pool/lock waits. A fast empty database is not representative evidence.

## Qualification harness (MVP-20)

`src/api/tests/performance/qualify.sh` runs the canonical-profile qualification end to end: resource-limited PostgreSQL and API containers, fixture from the seed scripts, load identities, a 2 min + 10 min run by default, and evidence into `specs/changes/020-performance/evidence/` (override with `OUT`). Use `WARMUP_S=10 MEASURE_S=30` for a quick smoke run. It needs Docker and the local `postgres:18.6` image, and never touches the development databases.

