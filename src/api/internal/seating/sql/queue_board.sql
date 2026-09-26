-- Active tickets only, keyset by join_order ($3 = previous last join_order).
SELECT t.id, t.branch_id, t.display_number, t.business_date::text, t.state, t.party_size, t.needs, t.source,
       g.id, g.label, t.called_until, dt.label, t.version, t.created_at, t.join_order
FROM queue_tickets t
LEFT JOIN seating_groups g ON g.id = t.seating_group_id
LEFT JOIN dining_tables dt ON dt.id = t.called_table_id
WHERE t.branch_id = $1 AND t.state IN ('waiting', 'called')
  AND ($2::text IS NULL OR t.state = $2)
  AND t.join_order > $3
ORDER BY t.join_order
LIMIT $4
