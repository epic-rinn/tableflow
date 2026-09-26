# TableFlow — Restaurant Queue and Order Management

Status: product concept for validation, not a final implementation specification.
For the current build scope and technical requirements, start at the [MVP knowledge map](README.md). The structured specs refine this research draft, including deferring point redemption and finite-stock inventory.
Research date: 26 September 2026.
Starting point: the owner's draft in the conversation; no existing draft file was present in the workspace. TableFlow is a working name taken from the project folder.

## 1. Product idea

TableFlow helps a dine-in restaurant manage one customer visit from joining the queue to collecting loyalty points after payment.

Customers join a queue, follow their position through a personal queue QR/link, receive a separate dining QR when staff assign a table, order from their phones, and present that dining QR at the cashier to retrieve their bill. Customers can optionally log in to earn points, progress through membership tiers, and receive discounts.

The restaurant uses a shared view of waiting parties, tables, orders, kitchen progress, and bills. Customers can complete the core journey without creating an account or installing an app.

**Proposed problem statement:** During busy service, uncertainty about queues and repeated handoffs between the entrance, table, kitchen, and cashier can create avoidable waiting and reconciliation work. TableFlow should keep the party, table, orders, and bill connected throughout the visit.

This is a problem hypothesis grounded in existing restaurant solutions. Desk research establishes that these workflows exist; it does not establish how frequently a specific independent restaurant experiences each problem or whether it would buy this product.

## 2. What restaurants already do

The following distinguishes documented practices from implications for this project. Vendor descriptions establish advertised capabilities, not independently measured efficiency gains or universal adoption across branches.

| Real example | Documented practice | Implication for TableFlow |
| --- | --- | --- |
| Sizzler Thailand and QueQ | Sizzler's FAQ directs customers to choose a restaurant branch through QueQ to join a queue. QueQ describes viewing queues and obtaining queue numbers remotely. | Digital waiting is an established local workflow. Make queue status easy to revisit and identify the correct branch. Sources: [Sizzler FAQ](https://www.sizzler.co.th/th/faqs), [QueQ](https://www.queq.me/queq---no-more-queue-line.html). |
| Wongnai POS Mobile Order, Thailand | Staff open a table, enter party size, and generate an ordering QR. Diners can order, review items, share the QR with companions, request help, and request checkout; submitted orders can print automatically. | The dining QR should belong to the current table visit, support a group, and connect to staff operations. Source: [Mobile Order workflow](https://www.wongnai.com/pos-articles/wongnai-pos-mobile-order). |
| Wongnai POS ecosystem, Thailand | The product page advertises table ordering, kitchen routing, bill-specific payment QR with payment verification, and loyalty features. Its pickup queue display concerns food collection, which differs from waiting for a table. | Several proposed capabilities already exist together. Integration alone is not a proven competitive advantage; distinguish seating queues from kitchen and pickup queues. Source: [Wongnai POS](https://www.wongnai.com/pos/). |
| MK Restaurants, Thailand | MKONE invites customers to register a membership card to collect points. | Optional membership has a local precedent. This source does not establish TableFlow's proposed tier or discount rules. Source: [MKONE membership](https://www.mkrestaurant.com/en/mkone). |
| TouchBistro, international comparison | Its reservations offering describes digital waitlists, table status, flexible floor plans, guest communication, and POS integration. | Seating depends on table readiness and party requirements, not simply advancing one global queue number. Source: [TouchBistro product guide, October 2024](https://www.touchbistro.com/wp-content/uploads/2024/09/TouchBistro-U.S.-Product-Guide-October-2024.pdf). |

**Market conclusion:** This is an established category. The opportunity to test is whether a focused browser-based workflow is easier to adopt for a particular type of restaurant than its existing process. Do not claim competitors lack these features, that restaurants universally use disconnected systems, or that TableFlow is cheaper without evidence.

## 3. Initial restaurant and users

Proposed pilot: one independent casual dine-in restaurant in Thailand with recurring peak-hour queues, several table sizes, shared group bills, and repeat customers. Thai and English customer screens and Thai baht pricing are sensible starting requirements to validate with that restaurant.

This scope fits a restaurant where customers sit first, order throughout the meal, and pay afterward. Counter-service order-and-pay shops, delivery businesses, and complex buffet packages have different operating rules and are outside the first release.

| User | Main job |
| --- | --- |
| Guest or dining party | Know when to return, get seated, order correctly, and settle the right bill. |
| Logged-in member | Earn and redeem benefits without slowing down dining. |
| Host or server | Manage waiting parties, assign suitable tables, and resolve exceptions. |
| Kitchen staff | See accepted orders, preparation notes, and their destination. |
| Cashier | Retrieve the correct bill, apply eligible benefits, and confirm payment. |
| Manager | Configure menus and rules, review adjustments, and measure service problems. |

One employee may perform several roles in a small restaurant; permissions should still distinguish routine actions from refunds and overrides.

## 4. Problems to validate

These are proposed failure modes to observe during real service, not claims that every Thai restaurant suffers from them.

| Potential problem | Operational consequence | Proposed response |
| --- | --- | --- |
| Guests cannot tell whether their turn is near. | Repeated questions, missed calls, or abandoned queues. | A live queue page with a clear called state and return policy. |
| A free table cannot accommodate the next party. | Confusion about apparent queue jumping or unsuitable seating. | Party-size-aware ordering and staff-confirmed table assignment. |
| Staff repeatedly take and re-enter orders. | Delayed submission or mismatched quantities and options. | Customer ordering connected to a staff/kitchen order view. |
| Different phones order for one table. | Duplicate submissions or fragmented bills. | One visit and bill with individually confirmed order submissions. |
| The cashier must determine which visit a customer is paying for. | Wrong-bill retrieval or extra clarification. | Retrieve the active bill using the dining QR. |
| Payment is assumed from an unverified screen or slip. | A bill closes without confirmed funds. | Staff verification initially; verified provider confirmation when integrated. |
| Membership identification happens too late or points are applied twice. | Disputes and manual corrections. | Link one member before settlement and award points once after payment. |

The product can reduce uncertainty and administrative delay. It cannot eliminate the underlying wait when all tables or kitchen stations are occupied.

## 5. Proposed customer journey

### Step 1 — Join the queue

- Scan the restaurant's entrance QR or ask staff to create a ticket.
- Enter party size and necessary seating requirements. Request a contact method only if needed for an optional notification channel.
- Receive a queue number plus a personal tracking link and QR. Offer a short retrieval code or printed ticket for guests without a usable phone.
- Allow cancellation. Limit repeated active tickets without forcing every guest to register.

The entrance QR opens the join page; the personal queue QR opens an existing ticket. A customer already using their phone follows the link directly instead of scanning their own screen.

### Step 2 — Track and answer the call

- Show the ticket, party size, status, last update, and number of eligible parties ahead.
- Explain that seating depends on party size and available tables. If an overall queue number is shown, do not imply it guarantees global first-in-first-out seating.
- Show an estimated wait range only when there is enough operational data; otherwise say that the estimate is unavailable.
- Staff call the party when a suitable table is ready. Display a configurable return window and explain the missed-call policy before joining.
- Keep the tracking page authoritative. Notifications are an enhancement; the MVP must not promise that a closed browser will alert a guest.

### Step 3 — Assign a table and create a dining visit

- Staff confirm the party is present and assign a suitable available table.
- Convert the queue entry into a seated visit and issue its separate dining QR.
- Guests who arrive when tables are free can be seated directly without creating an artificial waiting queue.
- Table assignment must prevent two staff members from allocating the same table concurrently.

### Step 4 — Order food

- Scan the dining QR to open the restaurant menu and current visit.
- Review prices, required options, quantities, availability, and any configured charges before confirming.
- Use a separate draft cart per device; confirmed orders join the same table bill. This avoids simultaneous edits to a shared draft cart.
- Show clear submission acknowledgement and distinguish accepted, preparing, ready, served, and cancelled items where staff actually maintain those states.
- Let customers request staff help. Allergy requests need staff acknowledgement; a free-text note alone must not imply that the kitchen can meet the request.
- Staff can place orders on behalf of customers who cannot or do not want to use a phone.

### Step 5 — Review the bill and pay

- Open the bill from the dining page or present the same dining QR to the cashier.
- The cashier retrieves that visit's itemized bill, confirms any changes, and applies eligible discounts before payment.
- The dining QR identifies a visit. It is not itself a bank payment QR or proof of payment.
- Initial payment is cashier-assisted: cash or the restaurant's existing payment method, with staff confirming receipt through the appropriate source. Record the payment method, amount, time, and confirming employee.
- If integrated online payment is added, create a separate payment request for the finalized amount and wait for verified provider confirmation before marking the bill paid.
- Freeze new orders during settlement; staff can reopen the bill if the party needs to continue ordering. Invalidate any earlier payment request when the payable amount changes.
- Payment closes the visit and disables ordering through its QR. The table becomes available only after guests leave and staff finish cleaning it.

### Step 6 — Earn points and improve membership tier

- Login is optional for queueing, ordering, and payment. It is required to attach benefits to a member account.
- Interpret “boost rank” as progressing through loyalty tiers, such as Silver and Gold, rather than jumping the seating queue.
- For the MVP, one identified member receives eligible points for the shared bill; agree this rule with the restaurant and make it visible before settlement.
- Award points once after confirmed payment. Cancelled orders do not earn points; refunds reverse associated earnings and tier credit under the configured policy.
- Separate redeemable points from qualifying spend or lifetime tier credit so redeeming points does not accidentally lower a member's tier.
- Define earning rate, eligible amount, thresholds, discount caps, expiry, and promotion stacking before launching membership. Values remain restaurant decisions, not assumed market standards.
- Show the applied discount and updated balance. Tier upgrades earned from this bill apply to future bills, avoiding circular recalculation of the current bill.

## 6. Core operating rules

### QR access and shared devices

| QR/link | Purpose | Lifetime and authority |
| --- | --- | --- |
| Entrance QR | Open the branch's queue registration page. | Long-lived; grants no access to existing visits. |
| Personal queue QR | Retrieve one party's queue status. | Valid for that queue entry; does not authorize ordering. |
| Dining QR | Access one active visit's ordering and bill page; retrieve the bill at checkout. | Issued at seating and invalid for new orders after closure. |
| Payment QR, if integrated | Start payment for a specific bill amount. | Governed by the payment provider and current bill version. |

Use unguessable access tokens. A photograph of a dining QR grants the same guest access while it remains valid, so staff need a way to revoke and replace it. Never expose member profiles, point redemption authority, or staff controls solely through that shared QR. Redeeming rewards requires the member's authenticated approval.

### States and exceptions

- Queue: waiting → called → seated, with cancellation and missed-call/no-show paths. Rejoining follows a published restaurant rule.
- Table: available → held for called party → occupied → cleaning → available. A missed call releases the hold.
- Visit: open → settling → paid/closed. Failed payment keeps it unsettled; reopening requires a controlled staff action.
- Orders: submitted → accepted → preparing → ready → served, with explicit rejection/cancellation handling. Staff approval is required for changes after preparation begins.
- Preserve the visit and bill when staff move a party to another table. Record who made the change.
- Prevent duplicate orders and duplicate point awards when requests are retried. Two guests attempting to settle one bill must not produce two successful settlements.
- Recheck menu availability when accepting an order, including a last-item conflict between customers.
- Record corrections, discounts, voids, and payment overrides with the responsible staff account and reason.
- During an outage, show that live status is unavailable; do not report an unacknowledged order as accepted. Use a documented manual ticket/order fallback and reconcile before billing.

Collect only the information required for service and optional membership. Keep marketing choices separate, restrict staff access, and establish retention and deletion rules before production use.

## 7. MVP and later scope

### MVP: one branch, complete dine-in visit

1. Guest queue registration and tracking, plus staff-created tickets and a call/no-show workflow.
2. Party size, table capacity/readiness, and manual assignment with conflict prevention.
3. A dining QR per visit, a menu with options and availability, and multiple order rounds on one bill.
4. A staff/kitchen screen with order acknowledgement and basic status updates.
5. Bill retrieval through the dining QR, itemized totals, configured charges/discounts, and cashier-confirmed settlement.
6. Optional member login, points ledger, basic tier progression, and one clear discount policy.
7. Staff permissions, audit history, and basic operational reporting.

The first prototype should prove queue → seating → ordering → payment. Basic loyalty completes the proposed MVP after settlement is reliable.

### Later, when justified by pilot evidence

- Automated payment-provider integration and reconciliation.
- LINE or SMS notifications and optional LINE login.
- Existing POS and kitchen-printer integrations.
- Split bills, multiple payers, complex refunds, merged tables, and multi-branch membership.
- Reservations, remote queue entry beyond the premises, advance ordering, and buffet time/package rules.
- Inventory purchasing, delivery aggregation, payroll, and accounting integrations.

For the pilot, choose a restaurant that can use this as its operational order/bill record with a basic kitchen screen. If an existing POS is mandatory, determine integration access first. Re-entering every order into another system could erase the proposed benefit. This concept does not establish a complete fiscal receipt or accounting solution.

## 8. Illustrative service scenario

This is a proposed scenario, not an observed case study.

A party of four joins a busy restaurant's queue and receives ticket A12. Their page shows eligible parties ahead and the return policy. A two-person table becomes free; a later party of two can be seated under the disclosed capacity rule without implying that A12 has been forgotten.

When a four-person table is ready, staff call A12, confirm arrival, assign table T08, and issue dining QR V104. Two friends scan it and submit separate orders to one bill. A repeated submit request does not create another order.

At the end of the meal, one member logs in and authorizes an eligible discount. The cashier scans V104, reviews the bill, and confirms payment. Points are awarded once, ordering closes, and T08 remains unavailable until staff clear and clean it. The next party receives a new visit and QR.

## 9. Validate before expanding

Observe at least two peak service periods at a willing pilot restaurant and interview its host, server, kitchen staff, cashier, manager, and several guests. Obtain permission before collecting operational data. These are proposed research activities; no interviews or field observations have been conducted for this document.

Questions to answer:

- How are queues currently recorded and called? How often do guests miss their turn or ask for updates?
- What prevents seating: capacity, cleaning, incomplete parties, reservations, or staff availability?
- Where are orders re-entered, corrected, or lost? How are stockouts communicated?
- How does the restaurant identify bills and verify payment today?
- How many guests prefer staff ordering, lack connectivity, or need another language?
- What POS, printers, and membership tools are already in use? Which must remain?
- Would the proposed workflow save enough staff effort to justify setup, training, and a subscription?

Capture a baseline and compare similar shifts during a small pilot. Suggested measures are queue abandonment/no-show rate, status questions per waiting party, seating delay after a suitable table is ready, order correction rate, accepted-order-to-kitchen visibility time, checkout duration, manual interventions, and member repeat visits over a longer period.

Operational correctness is a launch gate: no double-assigned tables, duplicate charges, duplicate loyalty awards, or successful ordering through a closed visit. Treat any occurrence as a defect to investigate, not an acceptable conversion tradeoff.

Set numerical improvement targets after measuring the baseline. Continue only if the restaurant and guests see a useful improvement without extra reconciliation work. If the current bottleneck is kitchen capacity alone, adding a queue or ordering interface may not address the primary problem.

## 10. Decisions still to confirm

- Pilot restaurant, party sizes, seating policy, and missed-call grace period.
- Whether the first deployment replaces a manual process or must integrate with an existing POS.
- Staff payment-verification procedure and eventual payment provider.
- Membership login method, earning rules, tier thresholds, and redemption policy.
- Tax/service-charge presentation, receipt needs, and refund responsibilities.
- Language needs, available devices/printers, and outage procedure.

The initial product promise is a connected, understandable dine-in visit. Commercial advantage, willingness to pay, and actual time savings remain hypotheses to prove with a restaurant pilot.
