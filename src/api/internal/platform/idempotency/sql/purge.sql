DELETE FROM idempotency_requests
WHERE id IN (SELECT id FROM idempotency_requests WHERE expires_at < now() LIMIT 5000)
