UPDATE member_tokens SET used_at = now() WHERE id = $1 AND used_at IS NULL
