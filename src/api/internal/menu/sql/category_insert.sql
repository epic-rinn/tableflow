INSERT INTO menu_categories (branch_id, name_th, name_en, sort) VALUES ($1, $2, $3, $4) RETURNING id
