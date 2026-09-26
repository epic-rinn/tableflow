SELECT id, password_hash, locale, email_verified_at IS NOT NULL FROM member_accounts WHERE email = $1
