UPDATE staff_accounts SET version = version + 1, updated_at = now() WHERE id = $1
