-- Unlocked read of the current table (lock order: tables before visits).
SELECT branch_id, table_id FROM visits WHERE id = $1
