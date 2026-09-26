INSERT INTO option_groups (branch_id, item_id, name_th, name_en, min_choices, max_choices, sort)
VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id
