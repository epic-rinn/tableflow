SELECT id, category_id, name_th, name_en, price_satang, sold_out, version, changed_revision
FROM menu_items WHERE branch_id = $1 AND retired_at IS NULL ORDER BY sort, id
