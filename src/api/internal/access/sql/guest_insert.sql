-- $1 capability, $2 generation, $3 token_hash, $4 lifetime, $5 capability expiry (nullable).
INSERT INTO guest_sessions (capability_id, generation, token_hash, expires_at)
VALUES ($1, $2, $3, LEAST(now() + $4::interval, COALESCE($5, 'infinity'::timestamptz)))
RETURNING id, expires_at
