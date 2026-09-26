-- Serialises policy edits per branch (staff admin lock order: branch first).
SELECT id FROM branches WHERE id = $1 FOR UPDATE
