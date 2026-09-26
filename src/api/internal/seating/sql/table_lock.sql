SELECT label, capacity, needs, state, active, version FROM dining_tables
WHERE id = $1 AND branch_id = $2
FOR UPDATE
