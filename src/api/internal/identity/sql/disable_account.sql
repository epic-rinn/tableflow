UPDATE staff_accounts SET status = 'disabled', version = version + 1, updated_at = now() WHERE id = $1
