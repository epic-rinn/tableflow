SELECT g.id, g.item_id, g.name_th, g.name_en, g.min_choices, g.max_choices,
       o.id, o.name_th, o.name_en, o.price_delta_satang
FROM option_groups g
LEFT JOIN menu_options o ON o.group_id = g.id AND o.retired_at IS NULL
WHERE g.item_id = ANY($1::uuid[]) AND g.retired_at IS NULL
