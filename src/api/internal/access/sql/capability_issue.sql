INSERT INTO capabilities (branch_id, kind, resource_id, token_hash, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING id
