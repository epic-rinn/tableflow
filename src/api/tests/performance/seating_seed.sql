-- M1 fixture per specs/quality/performance.md (disposable DB only): one
-- branch, 100 tables, 500 waiting parties, 200 active visits (plus 20 held
-- tables), 100,000 historical terminal tickets and 100,000 historical visits.
BEGIN;
INSERT INTO branches (id, name) VALUES ('00000000-0000-7000-8000-0000000000c1', 'Perf');
INSERT INTO seating_groups (branch_id, label, min_party, max_party) VALUES
  ('00000000-0000-7000-8000-0000000000c1', '1–2', 1, 2),
  ('00000000-0000-7000-8000-0000000000c1', '3–4', 3, 4),
  ('00000000-0000-7000-8000-0000000000c1', '5–6', 5, 6);

INSERT INTO dining_tables (branch_id, label, capacity, needs, state)
SELECT '00000000-0000-7000-8000-0000000000c1', 'T' || lpad(g::text, 3, '0'), (ARRAY[2, 4, 6])[1 + g % 3],
       CASE WHEN g % 10 = 0 THEN ARRAY['accessible'] ELSE '{}'::text[] END,
       CASE WHEN g <= 70 THEN 'occupied' WHEN g <= 80 THEN 'held' WHEN g <= 90 THEN 'cleaning' ELSE 'available' END
FROM generate_series(1, 100) g;

-- Historical tickets (terminal) over ~200 days, then 500 waiting + 10 called today.
INSERT INTO queue_tickets (branch_id, business_date, display_number, party_size, seating_group_id, source, state, terminal_at, created_at)
SELECT '00000000-0000-7000-8000-0000000000c1', current_date - (g / 500), 1 + g % 500, 1 + g % 6,
       (SELECT id FROM seating_groups WHERE branch_id = '00000000-0000-7000-8000-0000000000c1' AND (1 + g % 6) BETWEEN min_party AND max_party),
       'guest', (ARRAY['seated', 'cancelled', 'no_show'])[1 + g % 3], now() - (g / 500 || ' days')::interval, now() - (g / 500 || ' days')::interval
FROM generate_series(501, 100500) g;
INSERT INTO queue_tickets (branch_id, business_date, display_number, party_size, seating_group_id, source, needs)
SELECT '00000000-0000-7000-8000-0000000000c1', current_date, g, 1 + g % 6,
       (SELECT id FROM seating_groups WHERE branch_id = '00000000-0000-7000-8000-0000000000c1' AND (1 + g % 6) BETWEEN min_party AND max_party),
       'guest', CASE WHEN g % 25 = 0 THEN ARRAY['accessible'] ELSE '{}'::text[] END
FROM generate_series(1, 500) g;
-- Today's counter must match the tickets inserted above (the API allocates
-- display numbers from it).
INSERT INTO queue_counters (branch_id, business_date, last_number)
VALUES ('00000000-0000-7000-8000-0000000000c1', current_date, 500);

-- Holds for tables T071..T080 (called tickets).
WITH held AS (SELECT id, row_number() OVER (ORDER BY label) AS n FROM dining_tables WHERE state = 'held'),
     tk AS (SELECT id, row_number() OVER (ORDER BY join_order) AS n FROM queue_tickets WHERE state = 'waiting' AND party_size <= 2 LIMIT 10)
UPDATE queue_tickets q SET state = 'called', called_table_id = held.id, called_at = now(), called_until = now() + interval '5 minutes'
FROM tk JOIN held USING (n) WHERE q.id = tk.id;
INSERT INTO table_claims (table_id, branch_id, queue_ticket_id)
SELECT called_table_id, branch_id, id FROM queue_tickets WHERE state = 'called';

-- 100,000 historical visits on random tables, then 70 open visits.
INSERT INTO visits (branch_id, table_id, party_size, state, opened_at, ended_at)
SELECT '00000000-0000-7000-8000-0000000000c1', (SELECT id FROM dining_tables WHERE label = 'T' || lpad((1 + g % 100)::text, 3, '0')),
       2, 'departed', now() - (g / 500 || ' days')::interval, now() - (g / 500 || ' days')::interval + interval '1 hour'
FROM generate_series(1, 100000) g;
WITH occ AS (SELECT id FROM dining_tables WHERE state = 'occupied')
INSERT INTO visits (branch_id, table_id, party_size) SELECT '00000000-0000-7000-8000-0000000000c1', id, 2 FROM occ;
INSERT INTO table_claims (table_id, branch_id, visit_id)
SELECT table_id, branch_id, id FROM visits WHERE state = 'open';
COMMIT;
