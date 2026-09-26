-- Newest first; keyset on (occurred_at, id) within the range. $4 filters an
-- action or an action family ("settlement" matches "settlement.*").
SELECT a.id, a.occurred_at, a.action, a.resource_type, a.resource_id, a.reason, a.request_id, a.details,
       s.display_name
FROM audit_events a
LEFT JOIN staff_accounts s ON s.id = a.actor_staff_id
WHERE a.branch_id = $1 AND a.occurred_at >= $2 AND a.occurred_at < $3
  AND ($4::text IS NULL OR a.action = $4 OR a.action LIKE $4 || '.%')
  AND (a.occurred_at, a.id) < ($5, $6)
ORDER BY a.occurred_at DESC, a.id DESC
LIMIT $7
