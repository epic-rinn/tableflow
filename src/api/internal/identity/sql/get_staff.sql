SELECT a.id, a.branch_id, a.email, a.display_name, a.status, a.version, a.created_at,
       ARRAY(SELECT r.role FROM staff_roles r WHERE r.staff_account_id = a.id ORDER BY r.role)
FROM staff_accounts a
WHERE a.id = $1
