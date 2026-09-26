-- Resolve a presented session token to an active staff principal.
-- $1 token_hash, $2 idle timeout. One indexed lookup by unique token_hash.
SELECT s.id, a.id, a.branch_id, a.email, a.display_name,
       ARRAY(SELECT r.role FROM staff_roles r WHERE r.staff_account_id = a.id ORDER BY r.role),
       s.last_seen_at < now() - interval '1 minute' AS stale
FROM staff_sessions s
JOIN staff_accounts a ON a.id = s.staff_account_id
WHERE s.token_hash = $1
  AND s.revoked_at IS NULL
  AND s.expires_at > now()
  AND s.last_seen_at > now() - $2::interval
  AND a.status = 'active'
