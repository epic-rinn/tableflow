SELECT branch_id, version FROM menu_items WHERE id = $1 AND retired_at IS NULL FOR UPDATE
