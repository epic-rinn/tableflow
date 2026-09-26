INSERT INTO queue_tickets (branch_id, business_date, display_number, party_size, needs, seating_group_id, source)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id
