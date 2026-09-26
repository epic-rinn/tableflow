SELECT v.branch_id, v.state, v.bill_version, v.version, t.label
FROM visits v JOIN dining_tables t ON t.id = v.table_id
WHERE v.id = $1
