-- One ticket with its group position: waiting tickets in the same group
-- that joined earlier (partial index queue_tickets_waiting_group).
SELECT t.id, t.branch_id, t.display_number, t.business_date::text, t.state, t.party_size, t.needs, t.source,
       g.id, g.label, t.called_until, dt.label, t.version, t.created_at,
       CASE WHEN t.state <> 'waiting' THEN NULL
            WHEN t.seating_group_id IS NULL THEN
                (SELECT count(*) FROM queue_tickets o WHERE o.branch_id = t.branch_id AND o.state = 'waiting'
                   AND o.seating_group_id IS NULL AND o.join_order < t.join_order)
            ELSE
                (SELECT count(*) FROM queue_tickets o WHERE o.branch_id = t.branch_id AND o.state = 'waiting'
                   AND o.seating_group_id = t.seating_group_id AND o.join_order < t.join_order)
       END
FROM queue_tickets t
LEFT JOIN seating_groups g ON g.id = t.seating_group_id
LEFT JOIN dining_tables dt ON dt.id = t.called_table_id
WHERE t.id = $1
