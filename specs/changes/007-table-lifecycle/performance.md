# Performance: 007-table-lifecycle

See the shared M1 record in [005 performance](../005-queue-entry/performance.md) (move plans in [evidence/](evidence/)). Move, depart and close each lock one or two tables in ID order, then the visit (about 8–10 statements); rotation locks the visit, then its capability.
