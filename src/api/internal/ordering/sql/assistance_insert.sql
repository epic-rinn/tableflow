INSERT INTO assistance_requests (branch_id, visit_id, topic, note, raised_by)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (visit_id, topic) WHERE state IN ('open', 'acknowledged') DO NOTHING
RETURNING id
