-- $1 token_hash, $2 kind. Usable capability only.
SELECT id, branch_id, resource_id, generation, expires_at
FROM capabilities
WHERE token_hash = $1 AND kind = $2
  AND revoked_at IS NULL
  AND (expires_at IS NULL OR expires_at > now())
