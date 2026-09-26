UPDATE visits SET table_id = $2, version = version + 1, updated_at = now() WHERE id = $1
