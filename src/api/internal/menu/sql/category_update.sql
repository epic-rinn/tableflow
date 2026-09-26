UPDATE menu_categories SET name_th = $3, name_en = $4, sort = $5 WHERE id = $1 AND branch_id = $2 AND retired_at IS NULL
