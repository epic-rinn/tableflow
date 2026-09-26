SELECT count(*) FROM staff_accounts a
WHERE a.branch_id = $1 AND a.status = 'active'
  AND EXISTS (SELECT 1 FROM staff_roles r WHERE r.staff_account_id = a.id AND r.role = 'manager')
