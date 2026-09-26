-- A guest session is valid only for its capability's current generation.
SELECT g.id, c.id, c.branch_id, c.kind, c.resource_id, g.expires_at
FROM guest_sessions g
JOIN capabilities c ON c.id = g.capability_id
WHERE g.token_hash = $1
  AND g.revoked_at IS NULL AND g.expires_at > now()
  AND c.revoked_at IS NULL AND c.generation = g.generation
  AND (c.expires_at IS NULL OR c.expires_at > now())
