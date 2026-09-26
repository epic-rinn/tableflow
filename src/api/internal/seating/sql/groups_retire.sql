UPDATE seating_groups SET retired_at = now() WHERE branch_id = $1 AND retired_at IS NULL
