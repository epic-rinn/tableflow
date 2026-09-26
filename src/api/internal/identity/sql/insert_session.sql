-- $1 account, $2 token_hash, $3 absolute lifetime.
INSERT INTO staff_sessions (staff_account_id, token_hash, expires_at)
VALUES ($1, $2, now() + $3::interval)
RETURNING id, expires_at
