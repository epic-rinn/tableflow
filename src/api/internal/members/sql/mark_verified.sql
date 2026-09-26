UPDATE member_accounts
SET email_verified_at = coalesce(email_verified_at, now()), version = version + 1, updated_at = now()
WHERE id = $1
