-- All lines of a visit; rejected/cancelled are filtered in Go so the
-- unresolved count comes from the same read.
SELECT id, name_th, name_en, options, unit_price_satang, quantity, state
FROM order_lines WHERE visit_id = $1
ORDER BY created_at, id
