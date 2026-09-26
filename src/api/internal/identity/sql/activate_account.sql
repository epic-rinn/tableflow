UPDATE staff_accounts
SET password_hash = $2, display_name = $3, status = 'active',
    version = version + 1, updated_at = now()
WHERE id = $1 AND status = 'invited'
