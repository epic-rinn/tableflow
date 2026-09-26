-- M5 fixture addition (disposable DB only): 200,000 audit events spread over
-- 200 days so the audit viewer is measured on representative history.
INSERT INTO audit_events (branch_id, actor_staff_id, action, resource_type, resource_id, reason, request_id, details, occurred_at)
SELECT '00000000-0000-7000-8000-0000000000c1',
       (SELECT id FROM staff_accounts WHERE branch_id = '00000000-0000-7000-8000-0000000000c1' ORDER BY id LIMIT 1),
       (ARRAY['settlement.begun', 'settlement.confirmed', 'order_line.cancelled_late', 'visit.access_rotated', 'settlement.reopened'])[1 + i % 5],
       'visit', uuidv7(), CASE WHEN i % 5 IN (2, 3, 4) THEN 'seeded reason' END, 'seed-' || i,
       jsonb_build_object('amount_satang', i % 10000), now() - (i % 200) * interval '1 day' - (i % 86400) * interval '1 second'
FROM generate_series(1, 200000) i;
ANALYZE audit_events;
