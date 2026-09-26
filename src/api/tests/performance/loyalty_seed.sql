-- M4 fixture on top of billing_seed.sql (disposable DB only): 20,000
-- members with profiles, 60,000 member settlements with earn entries,
-- reversals for refunded ones, and the 35 settle-ready open visits claimed.
BEGIN;
INSERT INTO member_accounts (email, password_hash, locale, email_verified_at)
SELECT 'perf-member-' || i || '@perf.test', 'argon2id$perf', 'th', now() FROM generate_series(1, 20000) i;

CREATE TEMP TABLE m AS SELECT id, row_number() OVER (ORDER BY id) AS n FROM member_accounts WHERE email LIKE 'perf-member-%';
INSERT INTO member_profiles (branch_id, member_id) SELECT '00000000-0000-7000-8000-0000000000c1', id FROM m;

WITH picked AS (
    SELECT s.id, s.amount_satang, (row_number() OVER (ORDER BY s.id) % 20000) + 1 AS n
    FROM settlements s ORDER BY s.id LIMIT 60000
)
UPDATE settlements s
SET member_id = m.id, eligible_satang = (p.amount_satang * 80) / 100,
    points_earned = ((p.amount_satang * 80) / 100) / 10000, loyalty_policy_version = 0
FROM picked p JOIN m ON m.n = p.n
WHERE s.id = p.id;

INSERT INTO loyalty_ledger (branch_id, member_id, settlement_id, kind, points_delta, qualifying_delta_satang, policy_version, created_at)
SELECT branch_id, member_id, id, 'earn', points_earned, eligible_satang, 0, paid_at FROM settlements WHERE member_id IS NOT NULL;
INSERT INTO loyalty_ledger (branch_id, member_id, settlement_id, kind, points_delta, qualifying_delta_satang, policy_version, created_at)
SELECT s.branch_id, s.member_id, s.id, 'reversal', -s.points_earned, -s.eligible_satang, 0, r.created_at
FROM settlements s JOIN refunds r ON r.settlement_id = s.id WHERE s.member_id IS NOT NULL;

UPDATE member_profiles p
SET points_balance = l.points, qualifying_spend_satang = l.q,
    tier = CASE WHEN l.q >= 1500000 THEN 'gold' WHEN l.q >= 500000 THEN 'silver' ELSE 'base' END
FROM (SELECT member_id, sum(points_delta) AS points, sum(qualifying_delta_satang) AS q FROM loyalty_ledger GROUP BY member_id) l
WHERE l.member_id = p.member_id;

-- Claim the settle-ready open visits for the first 35 members.
WITH v AS (
    SELECT v.id, row_number() OVER (ORDER BY v.id) AS n FROM visits v
    WHERE v.state = 'open' AND NOT EXISTS (SELECT 1 FROM order_lines l WHERE l.visit_id = v.id AND l.state NOT IN ('served', 'rejected', 'cancelled'))
)
UPDATE visits SET member_id = m.id, member_claimed_at = now(), bill_version = bill_version + 1
FROM v JOIN m ON m.n = v.n WHERE visits.id = v.id;
COMMIT;
ANALYZE;
