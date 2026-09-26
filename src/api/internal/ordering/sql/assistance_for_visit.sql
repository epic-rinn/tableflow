SELECT a.id, a.visit_id, t.label, a.topic, a.note, a.state, a.version, a.created_at, a.acknowledged_at, a.resolved_at
FROM assistance_requests a JOIN visits v ON v.id = a.visit_id JOIN dining_tables t ON t.id = v.table_id
WHERE a.visit_id = $1
ORDER BY a.created_at DESC, a.id
LIMIT 20
