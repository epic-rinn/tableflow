# Performance: 013-refunds

Measured together with bills and settlement at the M3 gate: see [012 performance](../012-settlement/performance.md). With 100k settlements, the receipt list p95 was 6.2 ms and a receipt read 3.6 ms. Refunds are rare manager actions, and their statements use unique-key lookups. Plans are in [evidence/](evidence/).
