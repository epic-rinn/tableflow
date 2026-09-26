UPDATE order_lines SET state = $2, reason = $3, version = version + 1, updated_at = now() WHERE id = $1
RETURNING id, order_id, item_id, name_th, name_en, options, unit_price_satang, quantity, note, state, reason, version
