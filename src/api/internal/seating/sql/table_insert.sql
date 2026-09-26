INSERT INTO dining_tables (branch_id, label, capacity, needs) VALUES ($1, $2, $3, $4)
ON CONFLICT (branch_id, label) DO NOTHING
RETURNING id
