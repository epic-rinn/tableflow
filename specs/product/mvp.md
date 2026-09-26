# MVP scope

Status: specified; not implemented. Origin: [idea](../idea.md).

## Outcome

One Thai casual dine-in restaurant can connect a waiting party to a table, a shared bill, cashier-confirmed payment, and optional loyalty without mandatory customer registration. Guest UI supports Thai/English and THB. Initial deployment serves one branch; every business record still carries a branch boundary.

Deliver three runtime applications: a staff/manager admin panel, a customer PWA, and a shared Go API. Their source lives under `src/`; specs, docs, and AI workflows remain outside it. See [repository boundaries](../architecture/repository.md).

## Included

- Host-created or self-created queue tickets; personal tracking; party-size-aware staff seating; no-show handling; table cleaning and moves.
- Menu categories, required option groups, manual availability, multiple order rounds, a shared bill, staff-assisted ordering, and a kitchen screen.
- Itemized bill with configured charges; one payment for the full amount; cashier verification; manager-authorized full refund recording.
- Optional member accounts, paid-bill points, cumulative-spend tiers, and one tier discount per bill. Points redemption and expiry are deferred; the discount is unlocked by tier, not by spending points.
- Staff roles, audit events, PWA installability, visible offline/stale states, basic daily counts/totals.

## Excluded

Online payment integration, SMS/LINE push, reservations, split bills, partial refunds, combining occupied tables, cross-branch loyalty, inventory purchasing/finite-stock reservations, delivery, accounting, and POS/printer integration. Refund recording does not move money; external refunds remain a staff procedure. Availability is a sold-out toggle, not inventory accounting.

## Initial defaults to validate with the pilot

- First-come-first-served within compatible seating needs; manager overrides need reasons. No membership queue priority.
- Called-party return window: 5 minutes, configurable. No-show rejoins at the end. No automated wait-time estimate in MVP.
- Capacity groups: party sizes 1–2, 3–4, 5–6; larger or special arrangements require staff assistance. Configure actual table capacities before service.
- Loyalty example: 1 point per THB 100 of eligible post-discount food spend; Silver at THB 5,000 cumulative eligible spend (3% discount), Gold at THB 15,000 (5%). Base tier has no discount. These are editable pilot defaults, not Thai industry standards.
- Tax/service-charge settings require operator confirmation before taking real orders; no legal/accounting compliance claim is implied.

## Launch decisions

The operator must confirm seating groups/overrides, charge configuration, loyalty economics, verification/refund procedure, privacy retention, and usable hardware/network. Product work may proceed with explicit test defaults. Live launch waits for these operational decisions and the [quality gates](../quality/testing.md).
