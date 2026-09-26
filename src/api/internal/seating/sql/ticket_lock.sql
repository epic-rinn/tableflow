SELECT branch_id, state, party_size, needs, called_table_id, version, join_order
FROM queue_tickets WHERE id = $1 FOR UPDATE
