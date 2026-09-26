-- $1 account, $2 token_hash, $3 lifetime, $4 issuing staff (NULL for bootstrap).
INSERT INTO staff_activation_tokens (staff_account_id, token_hash, expires_at, created_by)
VALUES ($1, $2, now() + $3::interval, $4)
RETURNING expires_at
