UPDATE menu_options SET retired_at = now() WHERE branch_id = $1 AND retired_at IS NULL AND NOT (id = ANY($2::uuid[]))
