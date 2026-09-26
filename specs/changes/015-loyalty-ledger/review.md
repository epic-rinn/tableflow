# Review: 015-loyalty-ledger

Date: 2026-09-27 (M4 milestone gate, ADR-0004). Reviewer: Claude — **self-review**. Scope: eligible-spend and points calculation, ledger, earning inside confirm, reversal inside refund, member history routes and PWA account summary. Result: **pass**.

## Findings

| Severity | Location | Evidence | Impact | Resolution |
| --- | --- | --- | --- | --- |
| P3 | `member_loyalty.sql` | Filters on the second column of `(branch_id, member_id)` | Fine for one branch; slower with many branches | Recorded; add `member_profiles (member_id)` if multi-branch |
| P3 | Tier recalculation | Uses the current policy thresholds while amounts use the snapshot policy | Changing thresholds can move a member's tier on the next ledger change | Documented in the plan; consistent everywhere |

**Checked:**
- **Eligible spend (LOY-004):** fixtures cover exclusive, inclusive and discounted cases; service charge and its tax are excluded; points use floor.
- **Atomic earning (LOY-005):** earning happens in the confirm transaction, with the unique `(settlement, kind)` key.
- **Concurrent visits (LOY-A2):** two concurrent member confirms serialise on the profile, and the balance equals the ledger sum.
- **Refunds (LOY-A4/BIL-A6):** three concurrent refunds after a rate change give one reversal of the original 3 points; the profile returns to zero and Base; the visit stays paid.
- **History:** newest first with a keyset cursor, own rows only; no member session gets 401.
- **Balance:** never negative.
- **Load reconciliation:** the 20,000-profile fixture reconciles exactly after the run.

## Verification

| Command / test | Result |
| --- | --- |
| `make verify` (2026-09-27; macOS arm64, Go 1.27.1, Node 24.21.0, pnpm 12.6.0, postgres:18.6, Chrome) | **passed** on the first run: Go suite green (race, PostgreSQL, none skipped), admin 15/15, PWA 18/18 incl. `loyalty.spec.ts`, artifact check |
| MVP-15 tests | TestEligibleSpendFixtures, TestTiers, TestConcurrentMemberSettlements (LOY-A2), TestMemberRefundReversesOnce (LOY-A4), TestMemberLoyaltyHistory, TestLoyaltyContractConformance |
| Load | [performance](performance.md): about 55 req/s; member begin p95 16.5 ms, confirm 17.5 ms; ledger reconciles |

## Delivery decision

No open P0/P1. Point redemption, expiry and retrospective claims remain out of scope. Status: **done** (M4 gate).
