-- Claim a key inside the caller's transaction. A concurrent claim of the same
-- key blocks on the unique index until this transaction ends: on commit it
-- conflicts (and replays), on rollback it proceeds as the new owner.
INSERT INTO idempotency_requests (scope, operation, idem_key, request_hash, status_code, response, expires_at)
VALUES ($1, $2, $3, $4, 0, ''::bytea, now() + $5::interval)
ON CONFLICT (scope, operation, idem_key) DO NOTHING
RETURNING id
