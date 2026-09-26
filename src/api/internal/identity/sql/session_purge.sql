-- Sessions are useless once expired; keep 30 days for incident review, then
-- delete in bounded batches so the hourly job never holds long locks.
DELETE FROM staff_sessions
WHERE id IN (
    SELECT id FROM staff_sessions
    WHERE expires_at < now() - interval '30 days'
    LIMIT 5000
)
