# Menu, orders, and kitchen

Status: specified, unimplemented. Actors: dining guest, server, kitchen, manager.

## Requirements

- **MEN-001** Managers configure categories, Thai/English item names, satang prices, option groups with required/min/max choices, and a sold-out toggle. Retire items without deleting historical order snapshots.
- **ORD-001** Each device owns its draft cart. Submission identifies an active visit and provides item/option IDs, quantities (1–20 per line), optional notes (500 characters max), and the menu revision observed. Limit submissions to 50 lines and the request body to 64 KiB.
- **ORD-002** Go revalidates visit state, menu revision, selections, availability, and prices within the transaction. A changed relevant price/option/availability causes a conflict with refresh guidance, never a silently changed charge. Order creation snapshots names/options/unit amounts and submits the entire batch or nothing.
- **ORD-003** Submission is idempotent by actor/visit/operation/key and payload hash. Repeated keys with different bodies fail. An ambiguous network timeout is resolved by retrying the same request key, not by blindly creating a new order.
- **ORD-004** Kitchen sees submitted lines and explicitly accepts or rejects them. Accepted lines progress through preparing, ready, served. Rejections require a reason and remove the charge. A server may cancel an unprepared line with reason; preparing/ready/served cancellation requires manager authority and an audit event.
- **ORD-005** Guest and staff views show shared confirmed orders, totals, and states without sharing another device's draft cart. A staff-assisted order records the employee as its actor.
- **ORD-006** Guest may create one outstanding assistance request per visit/topic (help, allergy, checkout). Staff can acknowledge/resolve it. An allergy note is visibly unacknowledged until staff review it; no automatic claim that an item is safe.
- **ORD-007** Kitchen updates do not change bill prices. Cancelling/rejecting items changes totals only while the visit is open. Settling or paid visits reject new orders and financial edits; settlement rules define reopening and refunds.

Availability in MVP is a manual toggle. Serialize order validation against relevant menu updates; an accepted order before a sold-out change remains valid. There is no promise of finite-unit stock reservation until inventory is implemented.

## Acceptance scenarios

| ID | Given / When / Then |
| --- | --- |
| ORD-A1 | Given two phones at one visit, when each submits an order, then both appear exactly once on one bill (ORD-001/003/005). |
| ORD-A2 | Given an order committed but its response lost, when the same key/body is retried, then the original order is returned and no extra kitchen lines appear (ORD-003). |
| ORD-A3 | Given an item marked sold out or repriced after browsing, when submitted, then the whole submission fails with refresh guidance and no charge (ORD-002). |
| ORD-A4 | Given settlement racing with order submission, then either the order commits first and is included after settlement checks, or submission fails because settling has begun (ORD-007). |
| ORD-A5 | Given a rejected line, when the bill is read, then that line is clearly shown as non-chargeable and totals exclude it (ORD-004). |
| ORD-A6 | Given an allergy help request, when staff acknowledge it, then acknowledgement is visible without representing a dietary guarantee (ORD-006). |

Critical query paths: menu with options, orders with lines, and active kitchen lines. Fetch relationships in bounded batches; avoid one SQL query per item or option.
