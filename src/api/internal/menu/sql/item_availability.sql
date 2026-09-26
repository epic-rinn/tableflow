UPDATE menu_items SET sold_out = $2, changed_revision = $3, version = version + 1, updated_at = now() WHERE id = $1
