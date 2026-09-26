-- Keyset page over one branch. $2/$3 are the previous page's last key
-- ('-infinity' and the nil UUID for the first page), $4 the page size + 1.
SELECT a.id, a.branch_id, a.email, a.display_name, a.status, a.version, a.created_at,
       ARRAY(SELECT r.role FROM staff_roles r WHERE r.staff_account_id = a.id ORDER BY r.role)
FROM staff_accounts a
WHERE a.branch_id = $1 AND (a.created_at, a.id) > ($2, $3)
ORDER BY a.created_at, a.id
LIMIT $4
