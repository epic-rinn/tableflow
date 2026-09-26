# Performance and DB/API review: 004-member-access

Date: 2026-09-26 (M0 milestone gate). Reviewer: Claude (self-review). Same environment and fixture as [003](../003-guest-access/performance.md): 50,000 members, 150,000 member sessions, 100,000 tokens.

| Statement | Plan | Execution | Buffers |
| --- | --- | --- | --- |
| find_account (login/signup/reset) | unique email | 0.033 ms | 4 |
| authenticate_session | unique hash + PK | 0.031 ms | 4 |
| lock_usable_token (`FOR UPDATE`) | unique hash | 0.029 ms | 6 |
| revoke_member_sessions (reset) | partial `member_sessions_active_by_member` | 0.050 ms | 8 |
| revoke_open_tokens | partial unique open-token index | 0.049 ms | 6 |
| purge (hourly, sessions + tokens) | sequential scans, 5,000 batches | 28.3 ms | 24,069 |

Endpoint latency: GET `/members/me` 1000 samples, all 200, p50/p95/p99 0.28/0.42/1.45 ms, 114 bytes ([evidence](evidence/endpoint-latency.jsonl)). Member login and signup cost is dominated by argon2id (~22 ms per hash, measured for staff login in [002](../002-staff-access/performance.md)). Staff and members share one 4-slot hasher, so memory stays bounded across both. Mail is delivered asynchronously and adds no request latency; SMTP delivery time was not measured.
