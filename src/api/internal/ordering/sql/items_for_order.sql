-- Lock the ordered items FOR SHARE (ID order) so a concurrent menu change
-- waits for this validation, or this submission sees the change.
SELECT id, name_th, name_en, price_satang, sold_out, changed_revision, retired_at IS NOT NULL
FROM menu_items WHERE branch_id = $1 AND id = ANY($2::uuid[])
ORDER BY id
FOR SHARE
