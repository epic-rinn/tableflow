UPDATE table_claims SET queue_ticket_id = NULL, visit_id = $2 WHERE table_id = $1 AND queue_ticket_id = $3
