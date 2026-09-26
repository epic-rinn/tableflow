UPDATE option_groups SET name_th = $3, name_en = $4, min_choices = $5, max_choices = $6, sort = $7
WHERE id = $1 AND branch_id = $2 AND retired_at IS NULL
