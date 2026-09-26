-- Explicit revocation of derived sessions (defence in depth beside the generation check).
UPDATE guest_sessions SET revoked_at = now() WHERE capability_id = $1 AND revoked_at IS NULL
