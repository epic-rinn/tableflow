-- M3 fixture on top of ordering_seed.sql (disposable DB only): charge policy
-- versions, a frozen snapshot and settlement for each of the 100,000
-- departed visits (1,000 refunded), and every active line of the first 35
-- open visits served so they can settle during the load run.
BEGIN;
INSERT INTO charge_policies (branch_id, version, tax_mode, tax_bp, service_bp, created_by)
SELECT '00000000-0000-7000-8000-0000000000c1', v, 'exclusive', 700, 1000,
       (SELECT id FROM staff_accounts WHERE branch_id = '00000000-0000-7000-8000-0000000000c1' ORDER BY id LIMIT 1)
FROM generate_series(1, 20) v;

-- Historical snapshots mirror the calculation of 10% service and 7% tax.
INSERT INTO bill_snapshots (branch_id, visit_id, bill_version, policy_version, tax_mode, tax_bp, service_bp, discount_bp,
                            gross_satang, discount_satang, service_satang, tax_satang, total_satang, lines, created_by, created_at)
SELECT v.branch_id, v.id, v.bill_version, 20, 'exclusive', 700, 1000, 0, g.gross, 0, (g.gross * 1000 + 5000) / 10000,
       ((g.gross + (g.gross * 1000 + 5000) / 10000) * 700 + 5000) / 10000,
       g.gross + (g.gross * 1000 + 5000) / 10000 + ((g.gross + (g.gross * 1000 + 5000) / 10000) * 700 + 5000) / 10000,
       '[]'::jsonb, (SELECT id FROM staff_accounts WHERE branch_id = v.branch_id ORDER BY id LIMIT 1), v.opened_at
FROM visits v
JOIN LATERAL (SELECT coalesce(sum(unit_price_satang * quantity) FILTER (WHERE state NOT IN ('rejected', 'cancelled')), 0) AS gross
              FROM order_lines WHERE visit_id = v.id) g ON true
WHERE v.state = 'departed';

INSERT INTO settlements (branch_id, visit_id, snapshot_id, receipt_reference, amount_satang, method, verification_note, confirmed_by, paid_at)
SELECT b.branch_id, b.visit_id, b.id,
       'R-' || translate(upper(substr(md5(b.id::text), 1, 10)), '0189', 'WXYZ'),
       b.total_satang, (ARRAY['cash', 'bank_transfer', 'card'])[1 + (row_number() OVER ()) % 3], 'seeded', b.created_by,
       b.created_at + interval '50 minutes'
FROM bill_snapshots b;
UPDATE visits v SET paid_at = s.paid_at FROM settlements s WHERE s.visit_id = v.id;

INSERT INTO refunds (branch_id, settlement_id, amount_satang, reason, external_reference, recorded_by, created_at)
SELECT branch_id, id, amount_satang, 'seeded refund', 'REF-' || receipt_reference, confirmed_by, paid_at + interval '1 day'
FROM settlements ORDER BY id LIMIT 1000;

-- Serve the active lines of 35 open visits (the rest keep mixed kitchen states).
UPDATE order_lines SET state = 'served', version = version + 1
WHERE state IN ('submitted', 'accepted', 'preparing', 'ready')
  AND visit_id IN (SELECT id FROM visits WHERE state = 'open' ORDER BY id LIMIT 35);
COMMIT;
ANALYZE;
