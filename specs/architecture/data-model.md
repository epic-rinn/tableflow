# Data model and transaction design

Status: logical design, not executed DDL. Goose migrations become the source for implemented schema. IDs are UUIDs; ordering uses explicit sequences/timestamps plus a unique tie-breaker. All branch-owned relations include `branch_id` and use composite foreign keys where needed to prevent cross-branch relationships.

## Relations and access paths

| Relation | Main fields and invariants | Candidate indexes tied to queries |
| --- | --- | --- |
| branches / branch_policies | timezone, business settings, versioned charge/loyalty/seating policy | Primary keys; unique branch/policy version |
| accounts / staff_roles / member_profiles | normalized email, password hash, verified status; branch-scoped roles; member points/qualifying-spend totals | Unique normalized email; unique branch/account role; unique branch/member profile |
| sessions / capabilities | principal kind, hashed random secret, expiry/revocation; guest visit/ticket + capability generation | Unique secret hash; expiry cleanup index; resource/generation lookup |
| queue_counters | branch/business_date, last sequence | Unique branch/date; atomic increment |
| queue_tickets | daily display_sequence, global monotonic join_order, business_date, party_size, seating_group, needs, state, called_until, version | Unique branch/date/display_sequence; unique join_order; partial branch/group/join_order for waiting; partial called-deadline for called |
| dining_tables | label, capacity, supported needs, state, version | Unique branch/label; branch/state for board if measured useful |
| visits | branch, optional queue_ticket, state, bill_version, claimed_member, policy snapshot, timestamps | Unique non-null queue_ticket; branch/state/opened_at/id |
| table_claims | table_id, either queue_ticket_id or visit_id; branch | Primary/unique table_id; unique non-null visit_id and queue_ticket_id; check exactly one owner |
| menu_categories / menu_items / option_groups / options / item_option_groups | branch, display order, translated names, satang price/delta, sold_out, revision, option bounds | Branch/category/display order; relationship foreign keys; active menu lookup |
| orders / order_lines | visit, submit actor/key, line quantity, immutable name/option/price snapshot, state, notes | Orders: branch/visit/created_at/id; lines: order_id; partial branch/state/created_at/id for active kitchen |
| assistance_requests | visit, topic, state, acknowledgement actor/time | Unique visit/topic while unresolved; branch/state/created_at/id |
| bill_snapshots | visit, version, line/charge/discount totals, applied policies/member benefit | Unique visit/version; one final version referenced by settlement |
| settlements / refunds | visit, snapshot version, amount, method, actor, verification/reference; full reversal reference | Unique settlement visit; unique refund settlement; branch/paid_at/id for reports |
| loyalty_ledger | member, settlement, event kind, signed points and eligible-spend delta, policy version | Unique settlement/event kind; branch/member/created_at/id |
| idempotency_requests | scope/principal/operation/key, body hash, safe replay result, expires_at | Unique scoped key; expiry cleanup index |
| audit_events | branch, actor, action, resource, reason, request_id, occurred_at | Branch/occurred_at/id; resource lookup when justified |

These are index candidates, not instructions to create every index unconditionally. Primary/unique constraints are mandatory where specified. Validate performance indexes against real query predicates, ordering, plan evidence, write overhead, and data distribution. PostgreSQL does not automatically create every referencing foreign-key index.

## Constraints and integrity

Use `NOT NULL`, status/quantity/amount checks, uniqueness, and foreign keys for local invariants. Historical financial records must not cascade-delete when a menu item/account is retired. Keep order snapshots even if the source menu entity is unavailable.

`table_claims` is the exclusivity mechanism across both called holds and visits. A unique visit-only constraint cannot prevent two queue holds claiming one table. Table state and claim are updated in the same transaction and periodically checked for consistency. A paid visit retains its claim until departure.

Partial waiting/kitchen indexes avoid historical scans. A grouping count uses the configured seating group and join sequence; complicated requirements remain host-visible rather than an inaccurate algorithmic ETA. Keep active lists bounded and historical lists cursor-paginated.

## Locking and transaction boundaries

Default to PostgreSQL Read Committed with explicit row locks and guarded updates; choose stronger isolation only for a demonstrated invariant. Use a documented lock order across all paths: queue tickets (ID order) → tables (ID order) → visits (ID order) → member profiles (ID order) → menu/config rows (ID order) → child records. Operations may skip classes but must not acquire an earlier class after a later class. Update this order if implementation reveals an incompatible dependency.

- Call/seat: lock ticket if applicable, target table, check compatibility/state, replace or create claim, create visit if seating, update ticket/table, audit, commit.
- Move/depart: first read candidate table IDs without locking; lock tables in ID order, then visit, revalidate that its mapping is unchanged; otherwise abort/retry with a fresh read. Move claim atomically. This avoids visit→table order inversion.
- Submit order: lock visit, validate open state, lock relevant menu revision/items while validating, insert header/lines and idempotent result, update bill version, commit. No external calls inside the transaction.
- Begin settlement: lock visit, then member if claimed; validate states/version and snapshot charges/benefit. Confirm settlement repeats locks/revalidation, records payment and ledger changes in one transaction. Both order submission and settlement lock the same visit.
- Refund: lock visit/member, enforce unique reversal, append negative ledger credit and refund/audit records. Do not delete or reopen the paid bill.
- Role/menu/config changes follow the same lock rules if touching visit state. Do not hold database locks during email, network, or human verification.

Use bounded retries for deadlock/serialization failures only when the command is idempotent. A conflict from a changed business state is returned to the caller, not retried forever. Propagate request deadlines; set lock timeouts so polling is not starved by long transactions.

## Idempotency and secrets

All business mutations use an idempotency key scoped to authenticated actor/guest bootstrap identity, branch, operation, and resource when present. Store a canonical body hash and committed replay result in the same transaction as effects. Concurrent requests on the same key serialize; different payloads return conflict. A rolled-back transaction leaves no successful result.

Queue join requires a short-lived anonymous bootstrap session before submitting, so a guessed idempotency key cannot retrieve another guest's queue secret. Secret-bearing replay results (new queue/dining QR) must be encrypted at rest with a deployment key, never logged, and purged on expiry; capability lookup stores hashes only. An implementation may return a resource reference and securely re-issue access instead if it preserves retry semantics and multi-diner behavior; document that choice before coding.

Keep idempotency results for at least 24 hours and until the associated active visit/ticket terminates, whichever is later. Natural unique constraints on settlement, refund, loyalty events, and consumed tickets remain after replay records expire. Order clients must not automatically retry a expired-key command; reconcile its order history with staff.

## Migrations

Use timestamp-prefixed Goose SQL migrations with `-- +goose Up` and an intentional `Down` strategy. Never rewrite applied migrations. Run migrations as a release step, never on every API startup. Use expand/backfill/contract for incompatible changes; keep backfills bounded and resumable.

Validate fresh-database up and prior-release up. Test down only in disposable databases when it is genuinely lossless; destructive rollback requires an explicit recovery/restore plan. Normal production recovery uses a forward fix. Concurrent index operations need Goose's non-transactional mode in a separate migration, lock/statement timeout planning, and invalid-index recovery. No executable migration is claimed by this design document.
