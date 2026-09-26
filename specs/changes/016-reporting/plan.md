# Change: 016-reporting — Manager reporting and audit

Status: done (M5 gate passed 2026-09-27). Date: 2026-09-27. Scope owner: Claude. Task: MVP-16. Verification: M5 gate (ADR-0004).

## Problem and behavior

Maps OPS-001/002 and ADM-005 ([access/operations](../../features/05-access-pwa.md)). Managers see per-day summaries for a branch and date range: tickets and outcomes, visits, sales and refunds by method, and loyalty earned and reversed. They can page through the matching receipts, and read a redacted audit log.

## Decisions (reversible)

- **Business date:** the calendar date in the branch timezone (`branches.timezone`, default Asia/Bangkok). A range `from..to` (inclusive, at most 31 days) is converted to `[from 00:00, to+1 00:00)` in that timezone, then grouped by local date.
- **Report:** `GET /branches/{id}/reports/daily?from&to` (manager) returns `days[]` plus `totals`. Each metric is counted by the date of its own event:

  | Metric | Counted on |
  | --- | --- |
  | Tickets joined, and joined tickets now seated / no-show / cancelled | join date |
  | Visits opened | open date |
  | Settlements (count, amount by method) | paid date |
  | Refunds (count, amount by original method) | refund date |
  | Points earned / reversed, eligible spend | ledger date |

  Net sales = sales − refunds. There are five aggregate statements, one per source table, with no per-day queries.
- **Detail:** `GET /branches/{id}/settlements` gains `from`/`to` (same business-date rules, 31-day cap) for paging the receipts behind a report.
- **Audit:** `GET /branches/{id}/audit-events?from&to&action&cursor&limit` (manager) is a keyset page, newest first, with the actor's display name.
  - Details are passed through from an allowlist of keys that the application writes; unknown keys are dropped (defence in depth).
  - Tests assert that no token, hash or password material appears.
- **Indexes:** `queue_tickets (branch_id, created_at)`, `visits (branch_id, opened_at)`, `refunds (branch_id, created_at)` and `loyalty_ledger (branch_id, created_at)`. Each is justified by the report plans; the write cost is one index entry per row.
- **Navigation:** the sidebar is grouped into Service (host, kitchen, cashier, receipts) and Manage (reports, audit, tables, menu, charges, loyalty, staff).

## Verification map

| Requirement | Test | Status |
| --- | --- | --- |
| Reports reconcile to settlements, refunds and ledger | TestDailyReportReconciles | passing |
| Business date in the branch timezone | TestReportBusinessDate | passing |
| 31-day cap, validation, roles | TestReportBoundsAndRoles | passing |
| Audit viewer paginates, filters, is manager-only and redacted | TestAuditEventsViewer | passing |
| Receipt detail filtered by date range | TestReceiptLookup (extended) | passing |

## Implementation notes

- **Shared contract helper:** `testenv.CheckContract` validates responses against OpenAPI for new modules.
- **Browser teardown:** a `make verify` run once hung for 300 s after all admin tests passed, because a `next-server` outlived Playwright's teardown and was orphaned. It could not be reproduced alone (58 s, clean exit).
  - Both Playwright configs now use `gracefulShutdown` (SIGTERM, then a hard kill after 5 s).
  - The next full run passed and left no server behind.
  - The verify script does not kill port holders, which could be a developer's own server.
