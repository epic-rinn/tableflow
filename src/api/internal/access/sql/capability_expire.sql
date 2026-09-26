UPDATE capabilities
SET expires_at = LEAST(coalesce(expires_at, 'infinity'::timestamptz), now() + $3::interval)
WHERE kind = $1 AND resource_id = $2 AND revoked_at IS NULL
