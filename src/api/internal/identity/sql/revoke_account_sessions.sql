-- Expired sessions are already unusable; only live ones need revoking.
UPDATE staff_sessions SET revoked_at = now()
WHERE staff_account_id = $1 AND revoked_at IS NULL AND expires_at > now()
