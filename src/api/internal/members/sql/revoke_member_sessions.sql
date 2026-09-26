UPDATE member_sessions SET revoked_at = now()
WHERE member_id = $1 AND revoked_at IS NULL AND expires_at > now()
