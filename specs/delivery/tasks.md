# MVP implementation tasks

Status: M0 and M1 (MVP-01–07) done; M2 (MVP-08–10) implemented-unverified, gate pending; later tasks planned. Created: 2026-09-26.

This is the implementation queue for Claude. Codex maintains task scope and the Word report; Claude maintains execution status and evidence. The [roadmap](roadmap.md) groups milestones, while this file owns task order and status. Canonical feature specs remain the authority for behavior.

## Execution rules

1. Pick one task whose dependencies are `done`, or `implemented-unverified` within the same milestone ([ADR-0004](../decisions/0004-milestone-verification.md)). Do not start a new milestone before the previous milestone gate passed.
2. Create `specs/changes/<change-id>/plan.md`, `tasks.md`, and `review.md` before implementation. MVP-01 already has a prepared packet. Expand each task into file-level work after inspecting the code, not guessed scaffolds.
3. Deliver API, database, and appropriate UI together where specified. Include error/stale/offline states and authorization from the first slice, not only during final hardening.
4. Map named tests to the listed requirement IDs and acceptance scenarios. Follow [workflow](workflow.md), [testing gates](../quality/testing.md), and [performance evidence](../quality/performance.md).
5. Tests and reviews run at the milestone gate ([ADR-0004](../decisions/0004-milestone-verification.md)); each task still requires post-implementation [code review](../../.agents/skills/tableflow-code-review/SKILL.md). Changes to SQL, migrations, endpoints, or frontend requests also require [DB/API review](../../.agents/skills/tableflow-db-api-review/SKILL.md). Record whether review is self-review; never imply an independent reviewer ran.
6. Use `planned`, `in-progress`, `implemented-unverified`, `reviewed`, `done`, or `blocked`. For blocked work record the exact dependency and next action. `done` requires evidence, not checked boxes alone.

## Queue and dependencies

All implementation tasks are owned by Claude. MVP-22 additionally requires the restaurant operator/user; automation alone cannot close it.

| Task | Milestone | Change ID | Depends on | Status |
| --- | --- | --- | --- | --- |
| MVP-01 Runtime and verification foundation | M0 | `001-foundation` | None | done |
| MVP-02 Staff identity and access | M0 | `002-staff-access` | MVP-01 | done |
| MVP-03 Guest capabilities and mutation infrastructure | M0 | `003-guest-access` | MVP-02 | done |
| MVP-04 Member identity | M0 | `004-member-access` | MVP-02 | done |
| MVP-05 Tables and queue entry | M1 | `005-queue-entry` | MVP-03 | done |
| MVP-06 Calling and seating | M1 | `006-call-seat` | MVP-05 | done |
| MVP-07 Moves and table lifecycle | M1 | `007-table-lifecycle` | MVP-06 | done |
| MVP-08 Menu management and browsing | M2 | `008-menu` | MVP-07 | implemented-unverified |
| MVP-09 Shared-visit ordering | M2 | `009-ordering` | MVP-08 | implemented-unverified |
| MVP-10 Kitchen and assistance | M2 | `010-kitchen` | MVP-09 | implemented-unverified |
| MVP-11 Bill calculation and policies | M3 | `011-bill-calculation` | MVP-10 | planned |
| MVP-12 Cashier settlement | M3 | `012-settlement` | MVP-11 | planned |
| MVP-13 Receipts and full refunds | M3 | `013-refunds` | MVP-12 | planned |
| MVP-14 Member visit claim and tier snapshot | M4 | `014-member-claim` | MVP-04, MVP-13 | planned |
| MVP-15 Atomic loyalty and member history | M4 | `015-loyalty-ledger` | MVP-14 | planned |
| MVP-16 Manager reporting and audit | M5 | `016-reporting` | MVP-15 | planned |
| MVP-17 Installable PWA and cache isolation | M5 | `017-pwa` | MVP-16 | planned |
| MVP-18 Accessibility, locale and recovery audit | M5 | `018-journey-hardening` | MVP-17 | planned |
| MVP-19 Complete journey regression suite | M6 | `019-e2e` | MVP-18 | planned |
| MVP-20 Representative performance qualification | M6 | `020-performance` | MVP-19 | planned |
| MVP-21 Deployment and restore rehearsal | M6 | `021-operations` | MVP-20 | planned |
| MVP-22 Operator pilot sign-off | M6 | `022-pilot` | MVP-21 | planned |

## Task briefs

### MVP-01 — Runtime and verification foundation

- Deliver: independent Next.js admin/PWA shells, Go service, disposable PostgreSQL, pgx, Goose, pinned toolchains/lockfiles, origin routing, health/readiness, structured request IDs, and minimal OpenAPI contract. Add real development commands, a local verification gate (ADR-0003), database/migration tests, frontend build checks, and browser smoke checks.
- Read: [repository boundaries](../architecture/repository.md), [system](../architecture/system.md), [HTTP conventions](../api/http.md), and the [prepared plan](../changes/001-foundation/plan.md). Foundation for ACC-004/ADM-001; this task does not complete identity requirements.
- Accept: clean checkout starts all three applications; readiness fails safely when PostgreSQL is unavailable; both frontends build independently; migrations run on a fresh disposable database; production outputs exclude specs/docs/AI files. No staff/customer domain features yet.

### MVP-02 — Staff identity and access

- Deliver: session schema, safe initial-manager bootstrap, manager-provisioned staff activation, role management, staff login/logout, and role-aware admin navigation. Go enforces permissions, Origin/CSRF checks, expiry/revocation, authentication rate limits and private response headers.
- Map: ACC-002–004, ADM-001, staff subset of OPS-001; [access](../features/05-access-pwa.md), [admin](../features/06-admin.md), [security](../architecture/security.md).
- Accept: role-matrix API tests, forbidden cross-branch reads/writes, rejected forged Origin, revoked-role/session tests including in-flight requests, no public staff registration or customer cookie access to staff routes. Record session-query plans and authentication endpoint limits.

### MVP-03 — Guest capabilities and mutation infrastructure

- Deliver: anonymous session bootstrap, reusable multi-diner capability exchange, hashed tokens, revocation/rotation primitives, and persistent scoped idempotency with payload validation. Add a customer QR entry flow without restaurant-domain screens.
- Map: ACC-001/004 and the common [HTTP contract](../api/http.md); support later QUE-001, SEA-002 and ORD-003 rather than marking them complete here.
- Accept: raw tokens absent from logs/history/storage; fragment-to-POST exchange; isolated guest/member/staff cookies; simultaneous same-key requests return one committed result; different payload conflicts; failed writes do not leave false success records. Verify rotation invalidates derived sessions using real PostgreSQL.

### MVP-04 — Member identity

- Deliver: signup/login/logout, email verification and password reset through a local mail adapter, single-use token handling, member PWA account screens and session revocation on reset.
- Map: ACC-002/004; [access](../features/05-access-pwa.md). No points or visit claims yet.
- Accept: neutral account-discovery responses, expired/replayed token rejection, rate limits, no credential caching, coexistence with guest sessions, and cross-account access denial. Record deployment email-provider selection as unresolved until configured, not as a working production service.

### MVP-05 — Tables and queue entry

- Deliver: manager table/seating-group configuration, guest and assisted queue join, tracking QR/link, queue cancellation, and host waiting board. Use a monotonic join sequence independent of daily display numbering.
- Map: QUE-001/002 and waiting cancellation from QUE-004, table subset of OPS-001/ADM-005, ADM-002; [queue/seating](../features/01-queue-seating.md).
- Accept: QUE-A1/A2/A4, branch isolation, correct group position, same-key retry, tracking without other diners' data, bounded batched board queries, and hidden-tab/backoff polling tests. Explain stale state rather than promising precise wait times.

### MVP-06 — Calling and seating

- Deliver: call/hold/deadline, no-show and called cancellation, atomic seat/direct-seat, unique table claims, dining QR issuance, and host controls. Audit manager bypass reasons.
- Map: QUE-003/004, SEA-001/002, ADM-002.
- Accept: QUE-A3 and SEA-A1 with independent DB connections/barriers; no-show versus seat has one winner; repeated seating returns the original visit; deadlines do not silently release holds; incompatible or unfair direct seating is denied unless properly overridden.

### MVP-07 — Moves and table lifecycle

- Deliver: move, close-empty with reason, departure and cleaning/ready actions, and dining-token rotation. Preserve visit/orders/access during moves.
- Map: SEA-003/004, ACC-001, ADM-002.
- Accept: SEA-A2, ACC-A2, moving versus departure races, occupied-table rejection, and paid-table retention fixtures for SEA-A3. Use database fixtures to test paid states now; add the real cashier journey in MVP-12/19. No table merging or splitting.

### MVP-08 — Menu management and browsing

- Deliver: Thai/English categories/items/options, price and selection bounds, sold-out toggle, retirement, menu revision, manager editors and customer menu browsing.
- Map: MEN-001, menu subset of OPS-001/ADM-005; [ordering](../features/02-ordering.md).
- Accept: invalid option configuration denied, old snapshots preserved, batched item/options reads without N+1 queries, response size evidence, permission tests, and locale/currency display. Do not implement stock reservations.

### MVP-09 — Shared-visit ordering

- Deliver: per-device carts, validated all-or-nothing submissions, immutable price/name snapshots, shared confirmed orders and assisted staff ordering. Retain the request key after ambiguous failures.
- Map: ORD-001–003/005, order gating from ORD-007, ADM-002.
- Accept: ORD-A1/A2/A3; same key/different body conflict; menu-change versus submission races; quantity/body bounds; cross-visit denial; private no-store reads; distinct phone carts; query/transaction and retry evidence.

### MVP-10 — Kitchen and assistance

- Deliver: kitchen acceptance/rejection/preparation states, role-aware cancellation with reasons, allergy/help/checkout requests, staff acknowledgement and customer status views.
- Map: ORD-004/006/007, ADM-002/003.
- Accept: ORD-A5/A6; duplicate outstanding assistance coalesces; forbidden state/role transitions fail; preparing cancellation requires manager; settled fixtures prohibit financial changes. Measure active kitchen/assistance polling queries against historical data.

### MVP-11 — Bill calculation and policies

- Deliver: versioned charge/tax policies and manager editor, integer-satang calculation, bill API and itemised customer/cashier views. Implement the canonical discount calculation interface with nonmember inputs first.
- Map: BIL-001/003/004, policy subset of OPS-001/ADM-005; [settlement](../features/03-settlement.md).
- Accept: BIL-A2 for inclusive/exclusive tax, fractional rounding, rejected/cancelled lines and changed menu prices; server totals cannot be overridden by client input. Do not hard-code statutory rates or claim operator approval.

### MVP-12 — Cashier settlement

- Deliver: begin/reopen/confirm settlement with version checks, frozen ordering, receipt reference, exact recorded amount, and auditable externally verified payment. Enable nonmember settlement only until MVP-15 completes loyalty integration.
- Map: BIL-002/005/006/008, prepayment reopening from BIL-007, ORD-007, ADM-004.
- Accept: BIL-A1/A3/A4/A5, ORD-A4, duplicate/response-loss confirmations, unresolved kitchen lines, rollback invariants and retained table claims. Guest QR or uploaded slips must never authorize settlement. Add real paid-depart-clean coverage.

### MVP-13 — Receipts and full refunds

- Deliver: authorized historical receipt lookup, manager full-refund recording with external reference/reason, immutable original settlement, and cashier/manager UI.
- Map: BIL-007/008, ADM-004/005.
- Accept: repeated/concurrent refunds create one record; paid bills never reopen; cashier-only and guest users cannot refund; historical prices remain unchanged. Test nonmember refund now; member reversal is gated on MVP-15.

### MVP-14 — Member visit claim and tier snapshot

- Deliver: authenticated visit claim/detach flow, privacy-preserving claim status, versioned loyalty policy configuration, locked account/tier snapshot and discount preview. Keep member settlement unavailable until MVP-15 is done.
- Map: LOY-001–003, policy subset of OPS-001/ADM-005; [loyalty](../features/04-loyalty.md).
- Accept: LOY-A1/A3; another member cannot silently replace a claim; settling/paid claims immutable; shared QR reveals no identity/balance; one discount only. Test snapshot/account-lock behavior with real PostgreSQL.

### MVP-15 — Atomic loyalty and member history

- Deliver: earning ledger, balance/qualifying credit/tier updates within settlement, compensating full-refund entries using original policy, and paginated member history/progress. Enable member settlement only with the complete atomic integration.
- Map: LOY-004–007, complete member behavior for BIL-006/007.
- Accept: LOY-A2/A4, BIL-A3/A6, same-member concurrent visits, replayed refunds, inclusive-tax eligible-spend fixtures and ledger reconciliation. No point redemption/expiry or retrospective settled-bill claim.

### MVP-16 — Manager reporting and audit

- Deliver: branch/business-date summaries, sales/refunds by method, queue/visit/loyalty counts, paginated detail, audit viewer, and complete manager configuration navigation.
- Map: OPS-001/002, ADM-005; [access/operations](../features/05-access-pwa.md).
- Accept: reports reconcile to settlements and ledgers; 31-day cap and pagination enforced; unauthorized audit/report access denied; audit events omit secrets; query plans use representative history. Recheck configuration delivered in prior tasks rather than duplicating it.

### MVP-17 — Installable PWA and cache isolation

- Deliver: manifest/icons, generic offline page, versioned public-asset caching and safe update flow, install guidance, and admin/PWA origin isolation checks.
- Map: PWA-001–003, ADM-006.
- Accept: PWA-A1/A3 and ADM-A2; no API/QR/auth/private/RSC responses in service-worker caches; no background replay of mutations; update does not silently lose carts; no service worker on admin. Normal-browser use remains supported.

### MVP-18 — Accessibility, locale and recovery audit

- Deliver: complete Thai/English labels and error states, keyboard/status announcements, staff-assisted alternatives, consistent last-refresh indicators, polling cancellation/backoff and explicit timeout reconciliation across both applications.
- Map: PWA-003–005, ADM-006; this task audits and fills gaps, not postpones essential protections.
- Accept: PWA-A2, ADM-A4, reconnect/navigation/hidden-tab tests, anonymous/member switching privacy, keyboard core journey, readable currency, and failed-submit recovery without duplicate writes.

### MVP-19 — Complete journey regression suite

- Deliver: reproducible nonmember/member multi-device E2E fixtures and journey tests in `make verify` spanning queue, seating, kitchen, billing, rewards, departure and cleaning.
- Map: ADM-A3 and all cross-domain scenarios in [testing](../quality/testing.md).
- Accept: the seven required concurrency groups have real DB tests; lost-response/conflict/revocation/refund paths are exercised; requirement-to-test coverage gaps are closed or remain explicit blockers. No mock-only concurrency claims.

### MVP-20 — Representative performance qualification

- Deliver: seed/load tooling and sanitized SQL/HTTP evidence for queue, table board, menu, orders, kitchen, settlement, loyalty and reports under the canonical [performance profile](../quality/performance.md).
- Map: performance gates for all implemented requirements.
- Accept: actual hardware/versions, warmup/duration/cardinality, p50/p95/p99, response bytes, query counts, pool/lock waits and errors recorded; read plans and safe disposable transaction write plans reviewed; regression tests accompany measured optimizations. Missing required evidence blocks completion.

### MVP-21 — Deployment and restore rehearsal

- Deliver: reproducible staging deployment configuration/runbook, secret handling, HTTPS/origin setup, health monitoring, migration recovery procedure, backup/restore drill and email adapter configuration checks. No production deployment without approval.
- Map: ACC-002/004, PWA-001, release gates and [repository boundaries](../architecture/repository.md).
- Accept: restored database reconciles sample bills/ledger/claims; fresh and upgrade migrations tested; staged artifact inspection excludes specs/docs/AI files; security/session/cache checks hold on deployed origins. Record unresolved provider/hosting choices for the user.

### MVP-22 — Operator pilot sign-off

- Deliver: staff walkthrough, table/seating configuration confirmation, operator-approved charge/loyalty policies and receipt-verification procedure, recovery practice and pilot issue list.
- Map: all MVP requirements and [product scope](../product/mvp.md).
- Accept: user/operator reviews the real working journey and records actual outcomes; no invented interviews, sign-off or measured business benefit. Open launch-blocking issues keep the pilot blocked. Codex updates the original Word report only from the evidenced implementation and user-provided findings.
