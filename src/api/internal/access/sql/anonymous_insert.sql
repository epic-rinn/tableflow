INSERT INTO anonymous_sessions (token_hash, expires_at) VALUES ($1, now() + $2::interval)
RETURNING id, expires_at
