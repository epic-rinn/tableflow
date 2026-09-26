UPDATE assistance_requests
SET state = $2, version = version + 1,
    acknowledged_by = CASE WHEN $2 = 'acknowledged' THEN $3::uuid ELSE acknowledged_by END,
    acknowledged_at = CASE WHEN $2 = 'acknowledged' THEN now() ELSE acknowledged_at END,
    resolved_at = CASE WHEN $2 = 'resolved' THEN now() ELSE resolved_at END
WHERE id = $1
