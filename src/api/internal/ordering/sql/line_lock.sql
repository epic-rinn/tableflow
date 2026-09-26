SELECT branch_id, visit_id, state, version, unit_price_satang * quantity FROM order_lines WHERE id = $1 FOR UPDATE
