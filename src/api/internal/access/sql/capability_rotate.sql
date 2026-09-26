-- New secret and generation; sessions of older generations stop validating.
UPDATE capabilities SET token_hash = $3, generation = generation + 1, rotated_at = now()
WHERE kind = $1 AND resource_id = $2 AND revoked_at IS NULL
RETURNING id, generation
