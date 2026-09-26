INSERT INTO audit_events (branch_id, actor_staff_id, action, resource_type, resource_id, reason, request_id, details)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
