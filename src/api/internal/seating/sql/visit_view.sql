SELECT v.id, v.branch_id, v.state, v.party_size, v.needs, t.id, t.label, v.version, v.opened_at
FROM visits v JOIN dining_tables t ON t.id = v.table_id
WHERE v.id = $1
