# Change: 018-journey-hardening — Accessibility, locale and recovery audit

Status: implemented-unverified (M5 gate pending; `make verify` passes). Date: 2026-09-27. Scope owner: Claude. Task: MVP-18. Verification: M5 gate (ADR-0004).

## Problem and behavior

Maps PWA-003–005 and ADM-006. This task audits and fills gaps; it does not postpone protections that earlier tasks already delivered.

Findings from the audit (2026-09-27):
1. Guest screens are English-only apart from Thai menu names. PWA-004 requires Thai and English.
2. Guest hidden-tab polling, reconnect, keyboard-only ordering, member/anonymous switching and lost-response recovery are not covered by browser tests.
3. The cashier does not disable payment actions when offline (ADM-006). Retrying a confirmation after editing the note shows a confusing `IDEMPOTENCY_CONFLICT` (P3 from MVP-12).
4. Rotate-access accepts paid visits, which could restore read-only guest access after payment (P3 from MVP-12).
5. Dining and tracking links have no scannable or printable QR image (open item from MVP-06/07).

## Decisions (reversible)

- **Guest i18n:** a small in-house dictionary (`lib/i18n.tsx`) with Thai and English strings, a `LocaleProvider` and a TH/EN switch in the page header.
  - **Default language:** a saved choice (localStorage), otherwise the browser language (`th*` → Thai, anything else → English). `<html lang>` follows the choice.
  - Menu names already carry both languages. Currency stays THB in `th-TH` format (฿1,234.00).
  - The admin stays English, for staff, with Thai item names shown. This is recorded as a decision for the operator (MVP-22).
- **Recovery (unchanged mechanism, now tested):** idempotency keys survive ambiguous failures, and the UI explains that a retry cannot duplicate. The browser test drops the response after the server commits, retries, and asserts a single order.
- **Cashier:**
  - begin, confirm, reopen and detach are disabled when the bill poll reports a network failure;
  - `IDEMPOTENCY_CONFLICT` and `ALREADY_PAID` show a reconcile message ("check the bill before collecting again").
- **Rotate access:** limited to `open` and `settling` visits; paid visits keep their dining access revoked. This is a behaviour change covered by a Go test.
- **QR images:** the host share panel renders the dining or tracking link as a QR code (`qrcode-generator` 2.0.4, MIT, no dependencies; bundle cost measured at the gate) with a Print button and print-only styles. The token stays in the URL fragment.

## Verification map

| Requirement | Test | Status |
| --- | --- | --- |
| PWA-004 Thai/English, readable currency, keyboard journey | `journey.spec.ts` › Thai UI; keyboard-only order | passing |
| PWA-A2 hidden tab / reconnect | `journey.spec.ts` › polling | passing |
| PWA-003 failed submit recovers without duplicates | `journey.spec.ts` › lost response | passing |
| Anonymous/member switching privacy | `journey.spec.ts` › sign-out | passing |
| ADM-006 / ADM-A4 cashier offline disables payment | admin `cashier.spec.ts` › offline | passing |
| Rotate access refused after payment | TestRotateAccessOnlyBeforePayment | passing |
| QR image and print | admin `host.spec.ts` | passing |

## Implementation notes

- **Test issues found and fixed:**
  - The lost-response test first reported 4 Iced Coffee lines instead of 3. The database held exactly one guest order for that test, so there was no duplicate. The test counted nested list items twice; it now counts through the API.
  - The QR image's accessible name first collided with the host test's `/tracking link/` label query; it is now "Scannable QR code".
- **Language state:** errors that come from effects are stored raw and translated at render. This keeps the single-use verify effect from re-running on a language switch.
