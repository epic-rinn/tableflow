DELETE FROM anonymous_sessions
WHERE id IN (SELECT id FROM anonymous_sessions WHERE expires_at < now() - interval '1 day' LIMIT 5000)
