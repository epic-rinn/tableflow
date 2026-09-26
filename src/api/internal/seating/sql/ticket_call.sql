UPDATE queue_tickets
SET state = 'called', called_table_id = $2, called_at = now(),
    called_until = now() + make_interval(mins => $3), version = version + 1, updated_at = now()
WHERE id = $1
