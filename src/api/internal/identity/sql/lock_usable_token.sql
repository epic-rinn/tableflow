SELECT id FROM staff_activation_tokens
WHERE token_hash = $1 AND staff_account_id = $2
  AND used_at IS NULL AND revoked_at IS NULL AND expires_at > now()
FOR UPDATE
