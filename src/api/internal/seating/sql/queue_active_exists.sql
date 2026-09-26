SELECT EXISTS (SELECT 1 FROM queue_tickets WHERE branch_id = $1 AND state IN ('waiting', 'called'))
