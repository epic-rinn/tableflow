UPDATE capabilities SET revoked_at = now()
WHERE kind = $1 AND resource_id = $2 AND revoked_at IS NULL
RETURNING id
