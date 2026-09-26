UPDATE dining_tables SET state = $2, version = version + 1, updated_at = now() WHERE id = $1
