-- Re-check the acting session inside a mutation transaction. FOR SHARE
-- conflicts with revocation/role changes (which update these rows), so a
-- mutation either commits before a revocation or observes it and fails.
SELECT a.id, a.branch_id, a.email, a.display_name,
       ARRAY(SELECT r.role FROM staff_roles r WHERE r.staff_account_id = a.id ORDER BY r.role)
FROM staff_sessions s
JOIN staff_accounts a ON a.id = s.staff_account_id
WHERE s.id = $1
  AND s.revoked_at IS NULL
  AND s.expires_at > now()
  AND s.last_seen_at > now() - $2::interval
  AND a.status = 'active'
FOR SHARE OF s, a
