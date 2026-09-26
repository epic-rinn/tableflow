# Change: 022-pilot — Operator pilot sign-off

Status: **blocked** — needs the user and the restaurant operator. Claude does not invent walkthroughs, approvals or outcomes. Date: 2026-09-27.

## What sign-off requires ([MVP-22](../../delivery/tasks.md))

The operator reviews the real working journey and records actual outcomes. Open launch-blocking issues keep the pilot blocked. Codex updates the Word report only from evidenced implementation and user-provided findings.

## Operator checklist (to be completed by the user and operator)

| # | Item | Where | Default in the system today |
| --- | --- | --- | --- |
| 1 | Staff walkthrough: host, kitchen, cashier and manager each run the journey on real devices | admin and PWA | — |
| 2 | Tables, capacities, needs and seating groups match the floor | Admin › Tables & groups | Groups 1–2, 3–4, 5–6 |
| 3 | Charge policy: tax mode, VAT and service-charge rates confirmed with the operator's accountant | Admin › Charges & tax | Not configured (0%) |
| 4 | Loyalty economics confirmed: spend per point, Silver/Gold thresholds and discounts | Admin › Loyalty | Pilot defaults: ฿100/pt, Silver ฿5,000 (3%), Gold ฿15,000 (5%) |
| 5 | Receipt verification procedure: how cashiers verify transfers and cards in the restaurant's own systems; refund procedure | Operator SOP | App requires a verification note; slips are not proof |
| 6 | Recovery practice: lost connection, response lost after payment, reopen, refund, and new QR before payment | Admin + PWA | Behaviour tested in `make verify` |
| 7 | Language: staff UI is English with Thai item names; guest UI is Thai/English. Is English acceptable for staff? | — | English staff UI |
| 8 | Members with unconfirmed email may claim points. Is that acceptable? | — | Allowed |
| 9 | Hosting, public hostnames, backups (storage, retention, who restores), log and alert destinations | [Runbook](../../../docs/operations/deployment.md) | Open |
| 10 | Resend sending domain verified and API key provisioned; test emails delivered with tracking off | ADR-0005, runbook | Blocked (MVP-21) |
| 11 | Data retention and deletion for members, guest sessions and audit | Security spec | Open |
| 12 | Pilot issue list recorded from the walkthrough | This packet | — |

## Record of outcomes

_Empty until the walkthrough happens._
