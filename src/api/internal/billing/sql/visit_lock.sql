-- Same row lock as ordering (visit_lock): orders, cancellations and
-- settlement serialise on it (ORD-A4).
SELECT v.branch_id, v.state, v.bill_version, v.version, t.label
FROM visits v JOIN dining_tables t ON t.id = v.table_id
WHERE v.id = $1
FOR UPDATE OF v
