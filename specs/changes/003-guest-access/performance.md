# Performance and DB/API review: 003-guest-access

Date: 2026-09-26 (M0 milestone gate). Reviewer: Claude (self-review). Environment as in [002](../002-staff-access/performance.md): MacBook arm64, `postgres:18.6` without limits, Go 1.27.1, sequential client. Indicative, not load qualification.

Fixture `src/api/tests/performance/guest_member_seed.sql` (disposable `tableflow_perf`, `ANALYZE`d): 200,000 capabilities (≈100k visits + queue tickets, 90% revoked), 400,000 guest sessions, 20,000 anonymous sessions, 100,000 idempotency records.

| Statement | Plan | Execution | Buffers |
| --- | --- | --- | --- |
| capability_lookup (exchange) | unique `token_hash` index | 0.024 ms | 4 |
| guest_authenticate (every guest request) | unique session hash + capability PK | 0.029 ms | 8 |
| guest_revalidate (`FOR SHARE`) | PK + PK, LockRows | 0.037 ms | 10 |
| capability_rotate | unique `(kind, resource_id)` | 0.123 ms | 46 |
| guest_revoke_capability | partial `guest_sessions_active_by_capability` | 0.131 ms | 52 |
| anonymous_authenticate | unique hash | 0.017 ms | 4 |
| idempotency claim (new / existing key) | unique `(scope, operation, idem_key)` | 0.234 / 0.160 ms | 45 / 32 |
| idempotency lookup | same unique index | 0.041 ms | 8 |
| idempotency_purge (hourly, 5,000 batch) | sequential scan over 100k | 15.4 ms | 11,114 |
| purge_guest (hourly) | sequential scan over 400k | 36.3 ms | 11,190 |

A first `guest_authenticate` capture showed 9.5 ms because the *parameter-finding* subquery in my measurement script did a parallel sequential scan; the statement itself was re-planned with a literal hash (above). The hourly purges scan sequentially; they are bounded batches off the request path, and an index on `expires_at` exists for guest sessions if needed later.

Endpoint latency (direct to Go, sequential, [evidence](evidence/endpoint-latency.jsonl)):

| Route | Samples | Status | p50 / p95 / p99 ms | Bytes |
| --- | --- | --- | --- | --- |
| POST `/sessions/capability` | 500 | 201 | 0.88 / 1.82 / 2.80 | 164 |
| GET `/sessions/guest` | 1000 | 200 | 0.28 / 0.35 / 1.50 | 164 |
| POST `/sessions/anonymous` (new) | 500 | 201 | 0.59 / 0.92 / 1.94 | 45 |

Statement counts: exchange = 1 throttle upsert + 1 lookup + 1 insert; guest request = 1 authentication query; idempotent mutation = +1 claim and +1 complete (or claim + lookup on replay) inside the caller's transaction. Gaps: no concurrent HTTP mix, since polling routes arrive with M1.
