# Change: 008-menu — Menu management and browsing

Status: done (M2 gate). Date: 2026-09-26. Scope owner: Claude. Task: MVP-08. Verification: M2 gate ([ADR-0004](../../decisions/0004-milestone-verification.md)).

## Problem and behavior

Maps MEN-001 and the menu subset of OPS-001/ADM-005 ([ordering](../../features/02-ordering.md)).
- **Managers** edit the menu as one tree: categories → items → option groups → options, with Thai and English names, satang prices, required/min/max choices and ordering.
- **Sold out:** managers and kitchen toggle it per item.
- **Guests** browse the public menu.
- **Retirement:** retired entities disappear from the menu but stay referenced by historical order snapshots; nothing is deleted.

## Decisions (reversible unless noted)

- **Revisions:** `menus.revision` is the branch's menu revision, bumped by every change. Each item keeps `changed_revision`, the revision at which its price, option set or availability last changed. Order submission (MVP-09) rejects only lines whose item changed after the revision the guest loaded. Name-only and category moves don't bump it, since they don't change charges.
- **Option groups belong to one item** (no shared groups in MVP), so an option change maps to exactly one item's `changed_revision`. The logical data model's `item_option_groups` join is deferred.
- **Replacing the menu:** `PUT /branches/{id}/menu {expected_revision, categories:[…nested…]}` replaces the desired state atomically:
  - entries with an `id` are updated;
  - entries without one are created;
  - existing entries missing from the request get `retired_at` set.
  - Limits: body ≤1 MiB; ≤50 categories, ≤500 items, ≤10 groups per item, ≤20 options per group; names 1–80 characters; prices 0–10,000,000 satang; `0 ≤ min ≤ max ≤ option count` and `max ≥ 1`.
- **Availability:** `PATCH /menu-items/{id}/availability {expected_version, sold_out}` for manager or kitchen, with the item's own version (not the menu revision), so toggles on different items never conflict. It bumps the menu revision and the item's `changed_revision`.
- **Browsing:** `GET /branches/{id}/menu` is public and `no-store`. It returns the revision, `currency: "THB"`, and both Thai and English names (the client picks by locale). Active entities only, 4 statements (menu, categories, items, groups joined with options), no per-item queries. Response bytes are measured at the gate against the 500 KiB budget with 500 items.
- **Locks:** menu writes lock the `menus` row, then the affected items in ID order (menu/config class, after visits). Order submissions lock the visit, then their items `FOR SHARE`.

## Verification map

| Requirement | Test | Status |
| --- | --- | --- |
| MEN-001 editor, bounds, invalid option config | TestMenuReplaceValidation, TestMenuReplaceCreatesUpdatesRetires | passed (M2 gate) |
| Revisions: relevant vs irrelevant changes | TestChangedRevisionTracksChargeChanges | passed (M2 gate) |
| Availability toggle roles and versions | TestAvailabilityToggle | passed (M2 gate) |
| Public browse, batched, no retired entries | TestMenuBrowseBatched | passed (M2 gate) |
| Response size at 500 items | Gate measurement | measured |
