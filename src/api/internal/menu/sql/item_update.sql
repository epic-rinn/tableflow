-- $8 = new changed_revision (unchanged when the charge-relevant signature is equal).
UPDATE menu_items
SET category_id = $3, name_th = $4, name_en = $5, price_satang = $6, sort = $7, changed_revision = $8,
    version = version + 1, updated_at = now()
WHERE id = $1 AND branch_id = $2 AND retired_at IS NULL
