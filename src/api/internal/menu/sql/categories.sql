SELECT id, name_th, name_en FROM menu_categories
WHERE branch_id = $1 AND retired_at IS NULL ORDER BY sort, id
