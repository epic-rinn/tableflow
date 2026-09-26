INSERT INTO staff_accounts (branch_id, email, display_name, status)
VALUES ($1, $2, $3, 'invited')
ON CONFLICT (email) DO NOTHING
RETURNING id
