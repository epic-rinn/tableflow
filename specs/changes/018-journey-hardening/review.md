# Review: 018-journey-hardening

Date: 2026-09-27 (M5 milestone gate, ADR-0004). Reviewer: Claude — **self-review**. Scope: guest i18n, error translation, cashier offline and reconcile, rotate-access rule, QR images, journey tests. Result: **pass**.

## Findings

| Severity | Location | Evidence | Impact | Resolution |
| --- | --- | --- | --- | --- |
| P2 | Cashier (ADM-006) | Audit: payment actions stayed enabled while the bill could not refresh | A cashier could act on a stale bill | Fixed: offline banner, actions disabled; browser test |
| P2 | `seating/lifecycle.go` RotateAccess | Audit: allowed on paid visits | A new QR could restore read-only access after payment (BIL-008) | Fixed: open or settling only; TestRotateAccessOnlyBeforePayment |
| P3 | Field messages from the API | Server validation texts (e.g. "must be 1–500 characters") stay English in the Thai UI | Partially English messages for Thai guests on validation errors | Known codes are translated; field-level text is recorded for MVP-22 operator review |
| P3 | Admin language | Staff UI is English (Thai item names shown) | Staff who prefer Thai | Recorded as an operator decision for MVP-22 |

**Checked:**
- **Language:** Thai renders for Thai browsers and switches to English, with `<html lang>` and the choice persisted. THB format is `฿60.00`; the allergy safety text appears in Thai.
- **Keyboard:** an order can be placed with the keyboard alone, and focus moves into the sheet.
- **Polling (PWA-A2):** hidden-tab polling stops and resumes; the `online` event refetches.
- **Lost response (PWA-003):** a response lost after the server commits leads to a retry with one order. The database confirmed a single guest order; the first test failure was a counting bug in the test.
- **Sign-out:** removes all member data from the phone.
- **Cashier:** `IDEMPOTENCY_CONFLICT` shows a reconcile message.
- **QR:** the QR renders as React SVG (no HTML injection) with a print-only view.

## Verification

| Command / test | Result |
| --- | --- |
| `make verify` (2026-09-27; macOS arm64, Go 1.27.1, Node 24.21.0, pnpm 12.6.0, postgres:18.6, Chrome) | **passed**: Go suite green (race, PostgreSQL, none skipped), admin 16/16, PWA 25/25, artifact check |
| MVP-18 tests | PWA `journey.spec.ts` (5 tests); admin `cashier.spec.ts` offline, `host.spec.ts` QR; Go TestRotateAccessOnlyBeforePayment |
| Bundles | [performance](performance.md) |

## Delivery decision

No open P0/P1. Status: **done** (M5 gate).
