INSERT INTO menu_items (branch_id, category_id, name_th, name_en, price_satang, sort, changed_revision)
VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id
