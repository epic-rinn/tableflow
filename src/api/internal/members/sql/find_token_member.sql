-- Unlocked read to learn which account to lock first (account → token order).
SELECT member_id FROM member_tokens WHERE token_hash = $1 AND purpose = $2
