-- Synthetic, pessimistic staff-access fixture for query plans (disposable DB only).
-- 2 branches x 100 staff; 200,000 sessions (≈2 years of un-purged history,
-- mostly expired/revoked) plus 400 live sessions; 20,000 throttle buckets;
-- 50,000 audit events. Session token hashes are sha256 of 'tok-<n>'.
BEGIN;
INSERT INTO branches (id, name)
SELECT ('00000000-0000-7000-8000-00000000000' || b)::uuid, 'Branch ' || b FROM generate_series(1, 2) b;

INSERT INTO staff_accounts (id, branch_id, email, display_name, password_hash, status, created_at)
SELECT uuidv7(), ('00000000-0000-7000-8000-00000000000' || (1 + s % 2))::uuid,
       'staff' || s || '@perf.test', 'Staff ' || s, '$argon2id$dummy', 'active',
       now() - (s || ' minutes')::interval
FROM generate_series(1, 200) s;

INSERT INTO staff_roles (staff_account_id, role)
SELECT id, (ARRAY['host','kitchen','cashier','manager'])[1 + (row_number() OVER () % 4)::int] FROM staff_accounts;
INSERT INTO staff_roles (staff_account_id, role)
SELECT id, 'host' FROM staff_accounts WHERE NOT EXISTS (
  SELECT 1 FROM staff_roles r WHERE r.staff_account_id = staff_accounts.id AND r.role = 'host')
  AND random() < 0.3;

WITH a AS (SELECT id, row_number() OVER () AS n FROM staff_accounts)
INSERT INTO staff_sessions (staff_account_id, token_hash, created_at, last_seen_at, expires_at, revoked_at)
SELECT a.id, sha256(convert_to('tok-' || g, 'UTF8')),
       now() - (g % 700 || ' days')::interval,
       now() - (g % 700 || ' days')::interval,
       now() - (g % 700 || ' days')::interval + interval '12 hours',
       CASE WHEN g % 3 = 0 THEN now() - (g % 700 || ' days')::interval + interval '1 hour' END
FROM generate_series(1, 200000) g JOIN a ON a.n = 1 + g % 200;

-- 400 live sessions (2 per account), token 'live-<n>'.
WITH a AS (SELECT id, row_number() OVER () AS n FROM staff_accounts)
INSERT INTO staff_sessions (staff_account_id, token_hash, expires_at)
SELECT a.id, sha256(convert_to('live-' || g, 'UTF8')), now() + interval '11 hours'
FROM generate_series(1, 400) g JOIN a ON a.n = 1 + g % 200;

INSERT INTO auth_throttle (bucket, window_start, attempts)
SELECT 'login:ip:10.0.' || (g / 256) || '.' || (g % 256), now() - (g % 2880 || ' minutes')::interval, 1 + g % 9
FROM generate_series(1, 20000) g;

INSERT INTO audit_events (branch_id, actor_staff_id, action, resource_type, resource_id, request_id, occurred_at)
SELECT s.branch_id, s.id, 'staff.roles_changed', 'staff_account', s.id, 'perf-' || g, now() - (g || ' minutes')::interval
FROM generate_series(1, 50000) g JOIN LATERAL (SELECT id, branch_id FROM staff_accounts OFFSET g % 200 LIMIT 1) s ON true;
COMMIT;
ANALYZE;
