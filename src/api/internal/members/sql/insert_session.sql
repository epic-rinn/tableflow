INSERT INTO member_sessions (member_id, token_hash, expires_at)
VALUES ($1, $2, now() + $3::interval)
RETURNING id, expires_at
