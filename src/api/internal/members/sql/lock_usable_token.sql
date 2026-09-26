SELECT id FROM member_tokens
WHERE token_hash = $1 AND member_id = $2 AND purpose = $3
  AND used_at IS NULL AND revoked_at IS NULL AND expires_at > now()
FOR UPDATE
