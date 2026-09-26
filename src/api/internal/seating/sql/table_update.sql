UPDATE dining_tables
SET label = $2, capacity = $3, needs = $4, active = $5, version = version + 1, updated_at = now()
WHERE id = $1
