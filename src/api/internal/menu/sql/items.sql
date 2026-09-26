-- $2 (optional) restricts items to one category (large menus load per category).
SELECT id, category_id, name_th, name_en, price_satang, sold_out, version, changed_revision
FROM menu_items WHERE branch_id = $1 AND retired_at IS NULL AND ($2::uuid IS NULL OR category_id = $2)
ORDER BY sort, id
