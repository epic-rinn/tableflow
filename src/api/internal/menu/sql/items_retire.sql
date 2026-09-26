UPDATE menu_items SET retired_at = now(), changed_revision = $3, version = version + 1, updated_at = now()
WHERE branch_id = $1 AND retired_at IS NULL AND NOT (id = ANY($2::uuid[]))
