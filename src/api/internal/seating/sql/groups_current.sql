SELECT id, label, min_party, max_party FROM seating_groups
WHERE branch_id = $1 AND retired_at IS NULL ORDER BY min_party
