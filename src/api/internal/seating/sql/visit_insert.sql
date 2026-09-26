INSERT INTO visits (branch_id, table_id, queue_ticket_id, party_size, needs) VALUES ($1, $2, $3, $4, $5)
RETURNING id
