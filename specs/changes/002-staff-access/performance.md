# Performance and DB/API review: 002-staff-access

Date: 2026-09-26. Reviewer: Claude (self-review; no independent reviewer). Evidence: [evidence/](evidence/) (`plan-*.json`/`.txt`, `endpoint-latency.jsonl`).

## Environment and fixture

MacBook arm64 (10 CPU, 24 GiB), Docker 29.1.3, `postgres:18.6` without resource limits, API built with Go 1.27.1 on the same host, sequential Python client after warmup. Not the canonical 2 vCPU/1 GiB profile; timings are indicative only.

Fixture `src/api/tests/performance/staff_access_seed.sql` in a disposable `tableflow_perf` database, deliberately pessimistic: 2 branches × 100 staff; 200,000 historical sessions (about two years with no purge) plus 400 live ones; 20,000 throttle buckets; 50,000 audit events; `ANALYZE` after seeding. The pilot's real scale is under 100 staff and, with the 30-day session purge, a few thousand sessions.

## Routes and statements

| Route | Statements per request (excluding middleware auth) | Transaction/locks | Bounds |
| --- | --- | --- | --- |
| Any authenticated request | +1 `authenticate_session` (unique `token_hash`), +1 `touch_session` at most once a minute | none | — |
| GET `/sessions/current` | 0 | none | 186-byte body |
| GET `/branches/{id}/staff` | 1 `list_staff` keyset (roles via correlated array subquery) | none | limit ≤100; 25 rows ≈6.5 KiB, 100 rows ≈25 KiB |
| POST `/sessions/staff` | 2 `throttle_hit`, 1 `find_login_account`, argon2id outside any transaction, 1 `insert_session`, 1 `throttle_clear`, 1 authenticate | none | 16 KiB body; ≤4 concurrent hashes |
| POST `/staff/activate` | 1 throttle, 1 token lookup, argon2id before the transaction; then lock account → lock token → 2 updates + audit | short, 5 statements | as above |
| Staff-admin writes | lock branch → re-validate actor `FOR SHARE` → lock target `FOR UPDATE` → 2–4 writes → manager count → audit → read back | short, 7–9 statements, no external calls | 16 KiB body |
| Hourly maintenance | `throttle_purge`, `session_purge` (batches of 5,000) | autocommit | — |

## Query plans (EXPLAIN ANALYZE, warm cache)

| Statement | Plan shape | Execution | Buffers |
| --- | --- | --- | --- |
| authenticate_session (hit / miss) | Index scan on `staff_sessions_token_hash_key` + 200-row account scan | 0.108 / 0.046 ms | 13 / 6 |
| revalidate_actor (`FOR SHARE`) | Unique-hash index scan; LockRows | 0.115 ms | 19 |
| touch_session | PK index scan | 0.052 ms | 8 |
| list_staff first / middle page | Index scan on `staff_accounts_branch_page` with row-comparison condition; stops at LIMIT | 0.199 / 0.257 ms | 59 / 73 |
| revoke_account_sessions | Partial index `staff_sessions_active_by_account` | 0.208 ms | 62 |
| count_active_managers | Scans of 100 accounts / 259 roles | 0.065 ms | 6 |
| find_login_account | Unique email index | 0.030 ms | 4 |
| throttle_hit (upsert) | PK arbiter | 0.150 ms | 33 |
| session_purge (hourly) | Sequential scan of 200,400 sessions, deletes 5,000 | 19.5 ms | 8,258 |
| throttle_purge (hourly) | Bitmap scan on `auth_throttle_window` | 2.3 ms | 10,027 |

Small-table sequential scans (`staff_accounts`, `staff_roles`, a few hundred rows) are the planner's cheapest choice; no index is proposed. The session purge sequential scan runs hourly, and the purge itself keeps the table small; an `expires_at` index is not justified.

**Plan-driven change:** before the fix, `revoke_account_sessions` also revoked already-expired sessions: 668 rows and 9,380 buffers for one account in this fixture. Restricting it to `expires_at > now()` reduced that to 2 rows and 62 buffers, with the same semantics (expired sessions are already rejected).

## Endpoint latency (direct to Go, sequential)

| Route | Samples | Status | p50 / p95 / p99 ms | Bytes |
| --- | --- | --- | --- | --- |
| GET `/sessions/current` | 1000 | 200 | 0.44 / 1.09 / 1.63 | 186 |
| GET staff list, limit 25 | 1000 | 200 | 0.77 / 1.49 / 1.97 | 6,519 |
| GET staff list, limit 100 | 500 | 200 | 1.89 / 2.79 / 3.49 | 25,246 |
| POST `/sessions/staff` (argon2id) | 60 | 201 | 21.99 / 24.01 / 24.75 | 230 |
| 64 parallel logins, unknown accounts (dummy hash each) | 64 | 401 ×64 | median 199.5, max 348.9; wall 352 | — |

The first endpoint attempt returned 503s because the ad-hoc perf database lacked the app-role grants. That was a setup error, fixed with the same grants as `init.sql`. It showed the sanitized 503 path works (no SQL in responses; the cause is logged). Peak API RSS during the 64-way login burst was about 175 MiB, and bounded: at most 4 argon2id operations (≈19 MiB each) run at once and the rest queue. Queueing implies that a distributed flood of login attempts delays legitimate logins (≈22 ms × queue depth / 4); per-IP and per-email throttles limit individual sources only. Recorded as P3.

## Findings from this pass

- Fixed: expired-session revocation work (above).
- Fixed (P1, see [review](review.md)): client-controlled `X-Forwarded-For` through the Next.js proxy defeated per-IP throttling. No proxy is trusted by default now.
- Accepted: login queueing under a distributed flood (P3); fixed-window throttling allows up to 2× the limit across a window boundary (P3); per-IP limit shared by staff behind one NAT, raised to 100 per 10 min with the per-email limit (10) as the account protection.
- Gaps: no concurrent HTTP load mix (the canonical polling workload has no domain routes yet); no pool-wait metrics; single-host timings.
