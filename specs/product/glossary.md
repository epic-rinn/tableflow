# Domain vocabulary

| Term | Definition |
| --- | --- |
| Branch | The restaurant location and isolation boundary. One in the initial deployment. |
| Party | Guests dining together; represented first by a queue ticket or direct seating. |
| Queue ticket | One waiting attempt; its display number is not an authentication secret. |
| Seating group | Configured party-size band used to explain queue position. Special requirements may affect eligibility further. |
| Table claim | Exclusive ownership of a physical table for a called party or dining visit. |
| Visit | One party's dine-in session; persists across a table move. |
| Dining QR | Revocable capability to join/access that visit as a guest; never staff or member authority. |
| Order | One confirmed submission, containing immutable item/price/option snapshots. |
| Bill | The visit's computed payable total and finalized settlement snapshot. |
| Settlement | Staff-recorded full payment; does not itself transfer money. |
| Points ledger | Append-only history of awarded and reversed loyalty points. |
| Tier credit | Cumulative eligible spend used for membership rank, separate from points. |
| Idempotency key | Scoped client request identity used to replay a completed mutation without repeating its effects. |
| Requirement ID | Stable reference from acceptance behavior to tasks, tests, and review evidence. |
