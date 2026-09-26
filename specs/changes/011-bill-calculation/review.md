# Review: 011-bill-calculation

Date: 2026-09-26 UTC (M3 milestone gate, ADR-0004). Reviewer: Claude — **self-review**. Scope: `charge_policies` migration, `internal/billing` calculation, policy and bill reads, the admin charges editor and bill view, and the PWA bill panel. Result: **pass**.

## Findings

| Severity | Location | Evidence | Impact | Resolution |
| --- | --- | --- | --- | --- |
| P2 | `internal/billing/billing.go` SetChargePolicy | Review: the edit took the actor rows FOR SHARE, then the branch FOR UPDATE, while staff administration (`identity.adminTx`) locks branch → staff | A policy save racing a demotion of the same manager could deadlock (PostgreSQL aborts one; the user sees 503) | Fixed: no branch lock; the `(branch_id, version)` primary key arbitrates. Unique violation → `VERSION_CONFLICT`; covered by 4 concurrent edits in TestChargePolicyVersions |
| P2 (follow-up, pre-existing) | `internal/seating/config.go` ReplaceGroups, and staff-first paths that later take the branch row | Same inversion found while tracing the above: staff FOR SHARE → branch FOR UPDATE/SHARE versus adminTx branch → staff | Rare deadlock between seating-group edits (or host calls) and staff administration targeting the same staff member; it resolves as a 503 with no corruption | Not changed in M3 (outside scope). Owner: MVP-21 hardening. Adopt one global order (branch before actor rows) or drop branch locks where a constraint can arbitrate |
| P3 | Policy "eligible items" (BIL-004) | Every item is discount-eligible; there is no per-item eligibility | Matters only once tier discounts exist | Recorded assumption; revisit in MVP-14/15 |

Checked:
- Integer arithmetic with round-half-up once per aggregate (TestPerAggregateNotPerItem); hand-computed exclusive and inclusive fixtures (BIL-A2), including exact halves and fractions.
- Rejected and cancelled lines are excluded; later menu repricing leaves the bill unchanged; totals come only from Go.
- There is no statutory default: an unconfigured policy is labelled as such, and the UI requires operator confirmation.
- Policy versions are append-only, and frozen bills ignore later versions.
- Bill access: the visit's guest, or cashier/manager of the branch only. Other visits get 404; kitchen and host get 403. Reads are private and no-store.
- Resolving by QR works only for live tokens of the cashier's branch.

## Verification

| Command / test | Result |
| --- | --- |
| `make verify` (final, 2026-09-26T18:23Z UTC; macOS arm64, Go 1.27.1, Node 24.21.0, pnpm 12.6.0, postgres:18.6, Chrome) | **passed**: 142 Go tests (race, PostgreSQL, none skipped), admin 11/11 and PWA 15/15 browser tests, artifact check (2783 files / 100 sentinels) |
| Earlier M3 gate runs | Run 1 (`api-test-db`): route-parity test lacked the billing provider; guest→staff settlement was refused with 403 by the origin guard, where the test expected only 401; the revoked-QR check used a helper that fails on exchange. Tests fixed; product behaviour was correct. Run 2 (`make verify`): passed. Run 3, after the review fixes: the begin-versus-order race fixture had no lines and now correctly got `NOTHING_TO_SETTLE`; fixture fixed and the test repeated 5 times. Run 4: passed |
| MVP-11 tests | TestCalculateFixtures (BIL-A2), TestPerAggregateNotPerItem, TestLargeBillNoOverflow, TestChargePolicyVersions (including concurrent edits), TestBillUsesSnapshotsAndChargeableLines, TestBillAccess, TestBillStatementsConstant, TestBillingContractConformance; admin `cashier.spec.ts` (policy editor, bill 141.24), PWA `dining.spec.ts` (read-only bill) |
| Load | [performance](performance.md): bill p95 6.4–7.7 ms |

## Delivery decision

No open P0/P1. Tier discounts and member claim status arrive with MVP-14/15 through the same `Calculate` interface. Status: **done** (M3 gate).
