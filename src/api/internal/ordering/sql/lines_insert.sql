-- One statement for the whole batch.
INSERT INTO order_lines (branch_id, order_id, visit_id, item_id, name_th, name_en, options, unit_price_satang, quantity, note)
SELECT $1, $2, $3, l.item_id, l.name_th, l.name_en, l.options::jsonb, l.unit_price, l.quantity, l.note
FROM unnest($4::uuid[], $5::text[], $6::text[], $7::text[], $8::bigint[], $9::int[], $10::text[])
     WITH ORDINALITY AS l (item_id, name_th, name_en, options, unit_price, quantity, note, ord)
ORDER BY l.ord
