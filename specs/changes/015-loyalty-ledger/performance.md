# Performance and DB/API review: M4 (014 member claim, 015 loyalty ledger)

Date: 2026-09-27 (M4 milestone gate). Reviewer: Claude (self-review). Shared by [014](../014-member-claim/performance.md).

## Environment and fixture

Same host and profile as M1–M3: MacBook arm64, `postgres:18.6` without limits, pool 10. This is **not** the canonical 2 vCPU/1 GiB profile.

`src/api/tests/performance/loyalty_seed.sql` sits on top of the M3 fixture and adds:
- 20,000 members with branch profiles;
- 60,000 member settlements with earn ledger entries, plus reversals for their refunds (61,000 ledger rows in total);
- profile totals set from the ledger.

For the load run, the lines of the 35 remaining open visits were served and those visits claimed. Everything is `ANALYZE`d.

## Query plans ([014](../014-member-claim/evidence/), [015](evidence/))

| Statement | Plan | Execution | Buffers |
| --- | --- | --- | --- |
| visit_get (with member) | `visits_pkey` + 100-row table scan | 0.073 ms | 8 |
| profile_get / profile_lock | `member_profiles_branch_id_member_id_key` | 0.036 / 0.074 ms | 6 / 5 |
| loyalty_policy_current | scan of the empty table (pilot defaults) | 0.043 ms | 3 |
| member_loyalty | unique key (member as second column) + lateral policy | 0.069 ms | 14 |
| member_entries (page of 25) | `loyalty_ledger_by_member` + `settlements_pkey` | 0.093 ms | 27 |

`member_loyalty` filters by `member_id`, the second column of the `(branch_id, member_id)` key. That is fine for one branch, but a multi-branch deployment should add an index on `member_profiles (member_id)`. No index is added now because the pilot is single-branch.

## Load run ([evidence/loyalty-load.jsonl](evidence/loyalty-load.jsonl))

Workload, with 30 s warm-up and 120 s measured:
- 250 diner phones polling bills of member-claimed visits every 10 s;
- 10 cashiers reading bills;
- 5 receipt readers;
- 50 members reading their loyalty summary and history every 10 s;
- 5 cashier threads running begin → reopen cycles on claimed visits (member path: profile lock and tier snapshot), confirming each visit once.

That is about 55 requests/s.

| Route | Samples | Status | p50 / p95 / p99 ms |
| --- | --- | --- | --- |
| GET bill (diner, claimed) | 2994 | 200 ×2550, 401 ×444 | 3.95 / 6.69 / 8.59 |
| GET bill (cashier) | 983 | 200 ×983 | 4.97 / 8.04 / 9.94 |
| POST begin (member) | 572 | 200 ×572 | 11.38 / 16.45 / 18.71 |
| POST reopen (member) | 537 | 200 ×537 | 7.80 / 10.79 / 18.32 |
| POST confirm (member) | 35 | 201 ×35 | 12.98 / 17.50 / max 17.89 |
| GET /members/me/loyalty | 597 | 200 ×597 | 2.24 / 4.19 / 5.69 |
| GET /members/me/loyalty/entries | 597 | 200 ×597 | 1.54 / 2.73 / 4.30 |

The 401s are expected: payment revokes the dining sessions of visits confirmed during the run. API CPU averaged 4.9%, peak RSS was 56 MiB, and no errors were logged. After the run:
- all 20,000 profiles reconcile exactly with their ledger sums;
- each of the 35 new member settlements has exactly one earn entry.

A first attempt was invalid: my load script kept newlines in visit IDs, so its cashier requests never left the client. The script was fixed and the run repeated. The API was not at fault.

## Request cost

- **Bill read of a claimed open visit:** 2 statements more than M3 (profile tier and loyalty policy). Frozen bills read the member benefit from the snapshot.
- **Begin and confirm:** each adds the profile insert-or-lock. Confirm also adds the ledger insert, policy read and profile update.
- **Refund:** adds the profile lock, reversal insert, policy read and profile update.
- **Claim:** 6–8 statements.

## Gaps

- The canonical resource-limited profile was not used.
- Multi-branch member indexing is deferred, as above.
