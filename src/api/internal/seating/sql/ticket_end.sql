-- Terminal transition ($2 = cancelled | no_show | seated); clears any hold.
UPDATE queue_tickets
SET state = $2, called_table_id = NULL, called_until = NULL, terminal_at = now(),
    version = version + 1, updated_at = now()
WHERE id = $1
