SELECT id FROM seating_groups
WHERE branch_id = $1 AND retired_at IS NULL AND $2 BETWEEN min_party AND max_party
