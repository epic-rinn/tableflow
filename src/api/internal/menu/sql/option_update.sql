UPDATE menu_options SET name_th = $3, name_en = $4, price_delta_satang = $5, sort = $6
WHERE id = $1 AND branch_id = $2 AND retired_at IS NULL
