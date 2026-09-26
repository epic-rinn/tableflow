SELECT branch_id, email, display_name FROM staff_accounts
WHERE id = $1 AND status = 'invited'
FOR UPDATE
