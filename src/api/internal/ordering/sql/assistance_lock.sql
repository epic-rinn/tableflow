SELECT branch_id, visit_id, state, version FROM assistance_requests WHERE id = $1 FOR UPDATE
