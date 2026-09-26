SELECT id, expires_at FROM anonymous_sessions WHERE token_hash = $1 AND expires_at > now()
