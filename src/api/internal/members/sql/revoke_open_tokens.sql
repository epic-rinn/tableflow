UPDATE member_tokens SET revoked_at = now()
WHERE member_id = $1 AND purpose = $2 AND used_at IS NULL AND revoked_at IS NULL
