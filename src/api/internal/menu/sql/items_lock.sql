SELECT id FROM menu_items WHERE branch_id = $1 AND retired_at IS NULL ORDER BY id FOR UPDATE
