# Performance and DB/API review: M5 (016 reporting, 017 PWA, 018 journey hardening)

Date: 2026-09-27 (M5 milestone gate). Reviewer: Claude (self-review). Same host and profile as earlier milestones; **not** the canonical 2 vCPU/1 GiB profile.

## Fixture

The M4 perf database adds `src/api/tests/performance/reporting_seed.sql` (200,000 audit events over 200 days) and the reporting indexes migration. History covers:
- 100,070 settlements and 101,978 tickets (2026-03-09 to 2026-09-26);
- 1,000 refunds;
- 61,000 loyalty ledger entries;
- 202,854 audit events.

Everything is `ANALYZE`d. Measured window: 31 business days (2026-08-27 to 2026-09-26, Asia/Bangkok), about 15k settlements.

## Query plans ([evidence/](evidence/))

| Statement (31 days) | Plan | Execution | Buffers |
| --- | --- | --- | --- |
| report_tickets | Bitmap on `queue_tickets_by_created` + hash aggregate | 6.3 ms | 788 |
| report_visits | `visits_by_opened` | 4.4 ms | 1 |
| report_settlements | Bitmap on `settlements_by_branch` | 7.7 ms | 741 |
| report_refunds | scan of the 1,000-row table + `settlements_pkey` | 0.9 ms | 83 |
| report_loyalty | Bitmap on `loyalty_ledger_by_branch` | 6.4 ms | 316 |
| audit_page (first 50 of 203k) | `audit_events_branch_time` backward | 0.14 ms | 55 |
| audit_page with action family filter | same | 0.13 ms | 55 |

A full report is 5 aggregate statements plus the branch-timezone lookup, about 26 ms of SQL for 31 days, inside the ≤1 s report budget. Each new index is used, except `refunds_by_created`: the planner prefers a scan of 1,000 rows, and the index serves larger refund volumes.

## Client JavaScript (production build, signed-in routes, gzip)

| App / route | After M5 | Budget |
| --- | --- | --- |
| Admin `/host` (QR code) | 238.1 KiB | 400 KiB |
| Admin other workspaces incl. `/reports`, `/audit` | 203–235 KiB | 400 KiB |
| PWA `/t` and `/q` (translations) | 217.3 KiB | 250 KiB |
| PWA `/`, `/join`, `/account` | 193–195 KiB | 250 KiB |

CSS is 16.5 KiB (admin) and 10.5 KiB (PWA). Raw figures are in [018 evidence](../018-journey-hardening/evidence/).

- **QR library:** `qrcode-generator` added about 8 KiB gzip to `/host` only, not the 4 KiB first estimated in the 018 plan (corrected there).
- **Translations:** the Thai and English dictionary added about 12 KiB to the guest routes.

## Service worker (017)

The worker adds no runtime requests to API or page traffic: pages are network-only, and only hashed static assets are cache-first. Offline navigations cost one cache lookup.

## Gaps

- No load run: M5 adds no polling or hot write paths, since reports and audit are on-demand manager reads measured by plan.
- The resource-limited profile was not used.
