INSERT INTO member_accounts (email, password_hash, locale) VALUES ($1, $2, $3)
ON CONFLICT (email) DO NOTHING
RETURNING id
