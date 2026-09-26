-- All active groups with their active options in one statement (no N+1);
-- $2 (optional) restricts to items of one category.
SELECT g.id, g.item_id, g.name_th, g.name_en, g.min_choices, g.max_choices,
       o.id, o.name_th, o.name_en, o.price_delta_satang
FROM option_groups g
LEFT JOIN menu_options o ON o.group_id = g.id AND o.retired_at IS NULL
WHERE g.branch_id = $1 AND g.retired_at IS NULL
  AND ($2::uuid IS NULL OR g.item_id IN (SELECT id FROM menu_items WHERE category_id = $2 AND retired_at IS NULL))
ORDER BY g.item_id, g.sort, g.id, o.sort, o.id
