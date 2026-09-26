-- Serialises concurrent bootstrap attempts (constant advisory key).
SELECT pg_advisory_xact_lock(7302945116)
