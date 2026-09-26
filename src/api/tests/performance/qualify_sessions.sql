-- MVP-20 load identities (disposable qualification DB only). Reads the
-- tokens table `qt(kind, n, tok)` loaded by qualify.sh and stores only
-- SHA-256 digests, exactly as the API does.
BEGIN;
-- One staff account with every role for the synthetic staff clients.
INSERT INTO staff_accounts (id, branch_id, email, display_name, password_hash, status)
VALUES ('0198f0c0-0000-7000-8000-00000000f0aa', '00000000-0000-7000-8000-0000000000c1', 'qualify@perf.test', 'Qualify', 'argon2id$perf', 'active')
ON CONFLICT (id) DO NOTHING;
INSERT INTO staff_roles VALUES ('0198f0c0-0000-7000-8000-00000000f0aa', 'host'), ('0198f0c0-0000-7000-8000-00000000f0aa', 'kitchen'),
    ('0198f0c0-0000-7000-8000-00000000f0aa', 'cashier'), ('0198f0c0-0000-7000-8000-00000000f0aa', 'manager') ON CONFLICT DO NOTHING;
INSERT INTO staff_sessions (staff_account_id, token_hash, expires_at)
SELECT '0198f0c0-0000-7000-8000-00000000f0aa', sha256(convert_to(tok, 'UTF8')), now() + interval '11 hours' FROM qt WHERE kind = 'staff';

-- Queue trackers: a capability and a session per waiting ticket.
WITH w AS (SELECT id, branch_id, row_number() OVER (ORDER BY join_order) AS n FROM queue_tickets WHERE state = 'waiting'),
c AS (
    INSERT INTO capabilities (branch_id, kind, resource_id, token_hash)
    SELECT w.branch_id, 'queue', w.id, sha256(convert_to('qcap-' || w.id, 'UTF8')) FROM w JOIN qt ON qt.kind = 'queue' AND qt.n = w.n
    ON CONFLICT (kind, resource_id) DO UPDATE SET revoked_at = NULL
    RETURNING id, resource_id
)
INSERT INTO guest_sessions (capability_id, generation, token_hash, expires_at)
SELECT c.id, 1, sha256(convert_to(qt.tok, 'UTF8')), now() + interval '11 hours'
FROM c JOIN w ON w.id = c.resource_id JOIN qt ON qt.kind = 'queue' AND qt.n = w.n;

-- Diners: sessions spread over the active (open) visits.
INSERT INTO capabilities (branch_id, kind, resource_id, token_hash)
SELECT branch_id, 'visit', id, sha256(convert_to('vcap-' || id, 'UTF8')) FROM visits WHERE state = 'open'
ON CONFLICT (kind, resource_id) DO NOTHING;
WITH v AS (SELECT c.id AS cap, row_number() OVER (ORDER BY c.id) - 1 AS k, count(*) OVER () AS total
           FROM capabilities c JOIN visits vi ON vi.id = c.resource_id WHERE c.kind = 'visit' AND c.revoked_at IS NULL AND vi.state = 'open')
INSERT INTO guest_sessions (capability_id, generation, token_hash, expires_at)
SELECT v.cap, 1, sha256(convert_to(qt.tok, 'UTF8')), now() + interval '11 hours'
FROM qt JOIN v ON v.k = qt.n % v.total WHERE qt.kind = 'diner';

-- Members with loyalty history.
WITH m AS (SELECT member_id AS id, row_number() OVER (ORDER BY member_id) AS n FROM (SELECT DISTINCT member_id FROM loyalty_ledger) x)
INSERT INTO member_sessions (member_id, token_hash, expires_at)
SELECT m.id, sha256(convert_to(qt.tok, 'UTF8')), now() + interval '11 hours' FROM qt JOIN m ON m.n = qt.n WHERE qt.kind = 'member';
COMMIT;
