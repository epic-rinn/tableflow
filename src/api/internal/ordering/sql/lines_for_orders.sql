SELECT id, order_id, item_id, name_th, name_en, options, unit_price_satang, quantity, note, state, reason, version
FROM order_lines WHERE order_id = ANY($1::uuid[])
ORDER BY created_at, id
