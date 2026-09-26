-- Synthetic, pessimistic M0 guest/member fixture (disposable DB only).
-- 1 branch; 200,000 capabilities (≈100k historical visits + queue tickets,
-- 90% revoked); 400,000 guest sessions (2 per capability, mostly expired);
-- 20,000 anonymous sessions; 100,000 idempotency records; 50,000 members
-- with 150,000 sessions and 100,000 tokens. Known live secrets:
-- capability 'cap-live-<n>' (n ≤ 1000), member session 'msess-<n>'.
BEGIN;
INSERT INTO branches (id, name) VALUES ('00000000-0000-7000-8000-0000000000b1', 'Perf');

INSERT INTO capabilities (branch_id, kind, resource_id, token_hash, revoked_at, created_at)
SELECT '00000000-0000-7000-8000-0000000000b1', CASE WHEN g % 2 = 0 THEN 'visit' ELSE 'queue' END, uuidv7(),
       sha256(convert_to(CASE WHEN g <= 1000 THEN 'cap-live-' ELSE 'cap-' END || g, 'UTF8')),
       CASE WHEN g > 20000 THEN now() - (g % 365 || ' days')::interval END,
       now() - (g % 365 || ' days')::interval
FROM generate_series(1, 200000) g;

INSERT INTO guest_sessions (capability_id, generation, token_hash, created_at, expires_at, revoked_at)
SELECT c.id, 1, sha256(convert_to('gs-' || c.id || '-' || k, 'UTF8')), c.created_at,
       c.created_at + interval '12 hours', CASE WHEN c.revoked_at IS NOT NULL THEN c.revoked_at END
FROM capabilities c CROSS JOIN generate_series(1, 2) k;

INSERT INTO anonymous_sessions (token_hash, created_at, expires_at)
SELECT sha256(convert_to('anon-' || g, 'UTF8')), now() - (g % 30 || ' days')::interval, now() - (g % 30 || ' days')::interval + interval '24 hours'
FROM generate_series(1, 20000) g;

INSERT INTO idempotency_requests (scope, operation, idem_key, request_hash, status_code, response, created_at, expires_at)
SELECT 'guest:' || (g % 5000), 'orders.create', gen_random_uuid(), sha256(convert_to('req-' || g, 'UTF8')), 201,
       decode(repeat('ab', 300), 'hex'), now() - (g % 48 || ' hours')::interval, now() - (g % 48 || ' hours')::interval + interval '24 hours'
FROM generate_series(1, 100000) g;

INSERT INTO member_accounts (email, password_hash, locale, email_verified_at)
SELECT 'm' || g || '@perf.test', '$argon2id$dummy', CASE WHEN g % 3 = 0 THEN 'en' ELSE 'th' END,
       CASE WHEN g % 10 <> 0 THEN now() END
FROM generate_series(1, 50000) g;

WITH a AS (SELECT id, row_number() OVER (ORDER BY email) AS n FROM member_accounts)
INSERT INTO member_sessions (member_id, token_hash, created_at, last_seen_at, expires_at, revoked_at)
SELECT a.id, sha256(convert_to('msess-' || g, 'UTF8')), now() - (g % 400 || ' days')::interval,
       now() - (g % 400 || ' days')::interval, now() - (g % 400 || ' days')::interval + interval '30 days',
       CASE WHEN g % 4 = 0 THEN now() - (g % 400 || ' days')::interval END
FROM generate_series(1, 150000) g JOIN a ON a.n = 1 + g % 50000;

WITH a AS (SELECT id, row_number() OVER (ORDER BY email) AS n FROM member_accounts)
INSERT INTO member_tokens (member_id, purpose, token_hash, expires_at, used_at)
SELECT a.id, CASE WHEN g % 2 = 0 THEN 'verify' ELSE 'reset' END, sha256(convert_to('mtok-' || g, 'UTF8')),
       now() - (g % 200 || ' days')::interval, now() - (g % 200 || ' days')::interval
FROM generate_series(1, 100000) g JOIN a ON a.n = 1 + g % 50000;
COMMIT;
