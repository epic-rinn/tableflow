UPDATE member_sessions SET last_seen_at = now()
WHERE id = $1 AND revoked_at IS NULL AND last_seen_at < now() - interval '1 minute'
