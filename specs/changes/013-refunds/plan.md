# Change: 013-refunds — Receipts and full refunds

Status: done (M3 gate passed 2026-09-26 UTC). Date: 2026-09-27. Scope owner: Claude. Task: MVP-13. Verification: M3 gate (ADR-0004).

## Problem and behavior

Maps BIL-007, BIL-008, ADM-004 and ADM-005. Cashiers and managers look up historical receipts:
- by list (`GET /branches/{id}/settlements`, keyset by `paid_at, id`, optional `receipt_reference` filter);
- by ID (`GET /settlements/{id}`), returning the immutable snapshot and any refund.

Managers record one full refund per settlement: `POST /settlements/{id}/refund {reason, external_reference}` (reason 1–500 characters, reference 1–200, both required). The money moves outside the system; this only records it.

## Decisions

- **Refunds:** `refunds.settlement_id` is unique; the amount equals the settlement amount (full only). Locks: visit → settlement → refund insert. Duplicate or concurrent refunds leave one record; the loser gets `409 ALREADY_REFUNDED`. The original settlement is never modified or deleted, and the visit is never reopened.
- **Roles:** cashier-only staff and guests cannot refund (403 / 401). The refund is audited with its reason.
- **Members:** member point reversal waits for MVP-15; non-member refunds are complete now.

## Verification map

| Requirement | Test | Status |
| --- | --- | --- |
| One refund under concurrency and replay | TestConcurrentRefundOneRecord | passed |
| Roles (cashier, guest denied) | TestRefundRoles | passed |
| Paid bill never reopens; historical prices unchanged | TestReceiptImmutable | passed |
| Receipt lookup and pagination | TestReceiptLookup | passed |
