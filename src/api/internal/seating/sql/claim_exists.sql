SELECT EXISTS (SELECT 1 FROM table_claims WHERE table_id = $1)
