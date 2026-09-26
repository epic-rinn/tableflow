-- M2 fixture on top of seating_seed.sql (disposable DB only): a 500-item
-- menu (10 categories × 50 items, 2 option groups × 3 options each),
-- 100,000 historical orders with 1,000,000 served/cancelled lines, 12
-- active lines per open visit, 50,000 resolved and 30 open assistance
-- requests.
BEGIN;
INSERT INTO menus (branch_id, revision) VALUES ('00000000-0000-7000-8000-0000000000c1', 2)
ON CONFLICT (branch_id) DO UPDATE SET revision = 2;
INSERT INTO menu_categories (branch_id, name_th, name_en, sort)
SELECT '00000000-0000-7000-8000-0000000000c1', 'หมวด ' || c, 'Category ' || c, c FROM generate_series(1, 10) c;
INSERT INTO menu_items (branch_id, category_id, name_th, name_en, price_satang, sort, changed_revision)
SELECT '00000000-0000-7000-8000-0000000000c1', mc.id, 'อาหาร ' || mc.sort || '-' || i, 'Dish ' || mc.sort || '-' || i, 5000 + i * 100, i, 2
FROM menu_categories mc CROSS JOIN generate_series(1, 50) i
WHERE mc.branch_id = '00000000-0000-7000-8000-0000000000c1';
INSERT INTO option_groups (branch_id, item_id, name_th, name_en, min_choices, max_choices, sort)
SELECT branch_id, id, 'ตัวเลือก ' || g, 'Choice ' || g, CASE WHEN g = 1 THEN 1 ELSE 0 END, CASE WHEN g = 1 THEN 1 ELSE 2 END, g
FROM menu_items CROSS JOIN generate_series(1, 2) g WHERE branch_id = '00000000-0000-7000-8000-0000000000c1';
INSERT INTO menu_options (branch_id, group_id, name_th, name_en, price_delta_satang, sort)
SELECT branch_id, id, 'ตัวเลือก ' || o, 'Option ' || o, (o - 1) * 1000, o
FROM option_groups CROSS JOIN generate_series(1, 3) o WHERE branch_id = '00000000-0000-7000-8000-0000000000c1';

-- Historical: one order per departed visit, 10 lines each.
INSERT INTO orders (branch_id, visit_id, actor_kind, menu_revision, created_at)
SELECT branch_id, id, 'guest', 2, opened_at FROM visits WHERE state = 'departed';
INSERT INTO order_lines (branch_id, order_id, visit_id, item_id, name_th, name_en, unit_price_satang, quantity, state, reason, created_at, updated_at)
SELECT o.branch_id, o.id, o.visit_id, '0198f0c0-0000-7000-8000-000000000001', 'อาหาร', 'Dish', 8000, 1 + l % 3,
       CASE WHEN l = 10 THEN 'cancelled' ELSE 'served' END, CASE WHEN l = 10 THEN 'guest left' END, o.created_at, o.created_at
FROM orders o CROSS JOIN generate_series(1, 10) l;

-- Active: 3 orders × 4 lines for each open visit, mixed kitchen states.
INSERT INTO orders (branch_id, visit_id, actor_kind, menu_revision, created_at)
SELECT v.branch_id, v.id, 'guest', 2, now() - (k || ' minutes')::interval FROM visits v CROSS JOIN generate_series(1, 3) k WHERE v.state = 'open';
INSERT INTO order_lines (branch_id, order_id, visit_id, item_id, name_th, name_en, unit_price_satang, quantity, state, created_at)
SELECT o.branch_id, o.id, o.visit_id, (SELECT id FROM menu_items WHERE branch_id = o.branch_id LIMIT 1), 'อาหาร', 'Dish', 8000, 1,
       (ARRAY['submitted', 'accepted', 'preparing', 'ready'])[l], o.created_at
FROM orders o JOIN visits v ON v.id = o.visit_id AND v.state = 'open' CROSS JOIN generate_series(1, 4) l;

INSERT INTO assistance_requests (branch_id, visit_id, topic, state, raised_by, created_at, resolved_at)
SELECT branch_id, id, (ARRAY['help', 'allergy', 'checkout'])[1 + (row_number() OVER ())::int % 3], 'resolved', 'guest', opened_at, opened_at + interval '5 minutes'
FROM visits WHERE state = 'departed' LIMIT 50000;
INSERT INTO assistance_requests (branch_id, visit_id, topic, raised_by)
SELECT branch_id, id, 'help', 'guest' FROM visits WHERE state = 'open' LIMIT 30;
COMMIT;
