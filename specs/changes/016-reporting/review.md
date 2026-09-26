# Review: 016-reporting

Date: 2026-09-27 (M5 milestone gate, ADR-0004). Reviewer: Claude — **self-review**. Scope: business-date helper, reporting module (daily report, audit viewer), receipt date filter, reporting indexes, admin Reports and Audit screens, grouped navigation. Result: **pass**.

## Findings

| Severity | Location | Evidence | Impact | Resolution |
| --- | --- | --- | --- | --- |
| P2 | Playwright web-server teardown | A `make verify` run hung for 300 s after all admin tests passed: `next-server` was orphaned (PPID 1) and held the port for the next run | Local gate unreliable | Fixed: `gracefulShutdown` (SIGTERM, then a hard kill after 5 s) in both configs; four later full runs passed with no leftover server |
| P3 | `report_tickets.sql` | Tickets are grouped by join date with their *current* outcome | A ticket joined late at night and seated after midnight counts as seated on the join date | Documented in the plan; operators read "joined on" semantics |
| P3 | Audit allowlist | Unknown detail keys are dropped | A future audit key is hidden until added to the allowlist | Intentional (defence in depth); noted in the code comment |
| P3 | `src/admin/lib/dates.ts` | Default date presets use Asia/Bangkok | Wrong defaults only for a branch in another timezone (the API still uses the branch timezone) | Single-branch pilot; revisit for multi-branch |

**Checked:**
- **Reconciliation:** reports match the settlements, refunds and ledger tables in the test.
- **Business dates:** Bangkok midnight boundaries are tested (16:59:59Z vs 17:00:01Z).
- **Bounds and roles:** 31-day cap and validation (the 32-day, reversed, malformed and empty cases); manager-only (cashier 403, other branch 404).
- **Audit viewer:** keyset pagination covers all events; the action filter is validated; details are redacted (a planted token and password never appear).
- **Drill-down:** the receipts list is capped the same way.
- **Browser journey:** the Reports and Audit pages show the refund and its reason.

## Verification

| Command / test | Result |
| --- | --- |
| `make verify` (2026-09-27; macOS arm64, Go 1.27.1, Node 24.21.0, pnpm 12.6.0, postgres:18.6, Chrome) | **passed**: Go suite green (race, PostgreSQL, none skipped), admin 16/16, PWA 25/25, artifact check |
| MVP-16 tests | TestDailyReportReconciles, TestReportBusinessDate, TestReportBoundsAndRoles, TestAuditEventsViewer, TestReportingContractConformance, bizdate TestParse; admin `cashier.spec.ts` (reports and audit), `a11y.spec.ts` (new pages) |
| Plans | [performance](performance.md) |

## Delivery decision

No open P0/P1. Status: **done** (M5 gate).
