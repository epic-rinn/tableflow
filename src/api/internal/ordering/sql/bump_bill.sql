UPDATE visits SET bill_version = bill_version + 1, updated_at = now() WHERE id = $1
