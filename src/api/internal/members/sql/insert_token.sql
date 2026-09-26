-- $1 member, $2 purpose, $3 token_hash, $4 lifetime.
INSERT INTO member_tokens (member_id, purpose, token_hash, expires_at)
VALUES ($1, $2, $3, now() + $4::interval)
