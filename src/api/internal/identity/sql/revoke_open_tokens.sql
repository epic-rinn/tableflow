UPDATE staff_activation_tokens SET revoked_at = now()
WHERE staff_account_id = $1 AND used_at IS NULL AND revoked_at IS NULL
