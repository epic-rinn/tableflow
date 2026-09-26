-- Keyset page of a visit's orders ($2/$3 = previous last created_at/id).
SELECT id, actor_kind, created_at FROM orders
WHERE visit_id = $1 AND (created_at, id) > ($2, $3)
ORDER BY created_at, id
LIMIT $4
