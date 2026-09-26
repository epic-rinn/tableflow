-- A reset proves mailbox ownership, so it also verifies the email.
UPDATE member_accounts
SET password_hash = $2, email_verified_at = coalesce(email_verified_at, now()),
    version = version + 1, updated_at = now()
WHERE id = $1
