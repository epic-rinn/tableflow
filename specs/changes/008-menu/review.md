# Review: 008-menu

Date: 2026-09-26 (M2 milestone gate, [ADR-0004](../../decisions/0004-milestone-verification.md)). Reviewer: Claude — **self-review**. Scope: M2 commits `3ccf810`–`3e36ac3` and gate fixes (menu migration, `internal/menu`, admin `/menu`, sold-out toggles). Result: **pass with one P2 follow-up**.

## Findings

| Severity | Location | Evidence | Impact | Resolution |
| --- | --- | --- | --- | --- |
| P2 (open, owned) | Menu response size | A dense 500-item bilingual menu (3,000 options) is 613 KiB uncompressed, over the proposed 500 KiB | Slow first load on weak mobile networks for very large menus | Mitigated: category split `?category_id=` (62 KiB per category) and gzip on the menu route (80 KiB full). **Follow-up (owner Claude, MVP-20):** measure the operator's real menu. If it exceeds 400 KiB, switch the PWA to per-category loading with a per-line revision so separately loaded categories don't cause false MENU_CHANGED errors |
| P3 | Admin sold-out control | Gate run: a checkbox did not reflect clicks until the server answered | Confusing toggles | Fixed: the button shows server state and states the action |
| P3 (accepted) | Option groups | Groups belong to one item; shared groups are deferred | Repeated option setup across items | Documented in the plan and data model |

Checked: invalid bounds, prices, names and foreign IDs are rejected; omitted entries retire and history keeps them; renames don't invalidate carts while price and option changes do (per item); only manager or kitchen can toggle availability, guarded by the item version; the public read uses 5 statements (tested with a tracer); gzip applies only to the secret-free menu route.

## Verification

| Command / test | Result |
| --- | --- |
| `make verify` (final, 2026-09-26T16:13Z; macOS arm64, Go 1.27.1, Node 24.21.0, pnpm 12.6.0, postgres:18.6, Chrome 153) | **passed**: 120 Go tests (race, PostgreSQL, none skipped), admin 10/10 and PWA 14/14 browser tests, artifact check (2744 files / 88 sentinels) |
| Earlier M2 gate runs | Run 1: all ordering tests failed on an invalid test menu (max 2 choices, 1 option; correctly rejected by validation), fixture fixed. Run 2: sold-out checkbox did not reflect clicks. Run 3: reloading the dining page lost access. All fixed and re-run |
| MVP-08 tests | TestMenuReplaceValidation, TestMenuReplaceCreatesUpdatesRetires, TestChangedRevisionTracksChargeChanges, TestAvailabilityToggle, TestMenuBrowseBatched (5 statements; category split), TestMenuGzip, and admin `kitchen.spec.ts` (toggle, editor save): passed |

## Delivery decision

No open P0/P1; the P2 has an owner, trigger and measurement plan. Status: **done** (M2 gate).
