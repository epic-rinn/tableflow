-- In-transaction re-check for guest mutations; conflicts with rotation/revocation.
SELECT c.id, c.branch_id, c.kind, c.resource_id, g.expires_at
FROM guest_sessions g
JOIN capabilities c ON c.id = g.capability_id
WHERE g.id = $1
  AND g.revoked_at IS NULL AND g.expires_at > now()
  AND c.revoked_at IS NULL AND c.generation = g.generation
  AND (c.expires_at IS NULL OR c.expires_at > now())
FOR SHARE OF g, c
