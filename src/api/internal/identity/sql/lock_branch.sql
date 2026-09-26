-- Serialises staff administration per branch (config-class lock, taken first).
SELECT id FROM branches WHERE id = $1 FOR UPDATE
