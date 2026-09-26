SELECT branch_id, table_id, state, party_size, needs, version FROM visits WHERE id = $1 FOR UPDATE
