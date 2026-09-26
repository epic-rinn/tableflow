# Performance: 006-call-seat

See the shared M1 record in [005 performance](../005-queue-entry/performance.md) (fairness, visit and claim plans in [evidence/](evidence/)). Call and seat each run one short transaction: actor re-validation, ticket lock, table lock, fairness check, claim insert or conversion, state updates, capability issue, audit on override, plus the idempotency claim and completion (about 10–12 statements). No external calls.
