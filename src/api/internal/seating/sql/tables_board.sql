-- Whole floor in one statement (≤100 tables in the pilot), with each claim.
SELECT t.id, t.label, t.capacity, t.needs, t.state, t.active, t.version,
       c.queue_ticket_id, q.display_number, c.visit_id
FROM dining_tables t
LEFT JOIN table_claims c ON c.table_id = t.id
LEFT JOIN queue_tickets q ON q.id = c.queue_ticket_id
WHERE t.branch_id = $1
ORDER BY t.label
LIMIT 101
