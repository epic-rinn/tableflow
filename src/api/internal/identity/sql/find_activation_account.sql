-- Unlocked read to learn which account to lock first (account → token order).
SELECT staff_account_id FROM staff_activation_tokens WHERE token_hash = $1
