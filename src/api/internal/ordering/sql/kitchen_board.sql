-- Active lines only (partial index order_lines_kitchen), with table context.
SELECT l.id, l.order_id, l.visit_id, t.label, l.name_th, l.name_en, l.options, l.quantity, l.note, l.state, l.version, l.created_at
FROM order_lines l
JOIN visits v ON v.id = l.visit_id
JOIN dining_tables t ON t.id = v.table_id
WHERE l.branch_id = $1 AND l.state IN ('submitted', 'accepted', 'preparing', 'ready')
  AND ($2::text IS NULL OR l.state = $2)
  AND (l.created_at, l.id) > ($3, $4)
ORDER BY l.created_at, l.id
LIMIT $5
