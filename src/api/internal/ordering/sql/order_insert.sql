INSERT INTO orders (branch_id, visit_id, actor_kind, actor_staff_id, actor_guest_session_id, menu_revision)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, created_at
