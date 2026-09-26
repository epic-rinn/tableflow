DELETE FROM guest_sessions
WHERE id IN (SELECT id FROM guest_sessions WHERE expires_at < now() - interval '30 days' LIMIT 5000)
