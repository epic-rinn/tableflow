-- Guest capabilities, guest/anonymous sessions and persistent idempotency (MVP-03).

-- +goose Up
CREATE TABLE capabilities (
    id          uuid        PRIMARY KEY DEFAULT uuidv7(),
    branch_id   uuid        NOT NULL REFERENCES branches (id),
    kind        text        NOT NULL CHECK (kind IN ('queue', 'visit')),
    -- Queue ticket or visit ID; foreign keys arrive with those tables (MVP-05/06).
    resource_id uuid        NOT NULL,
    generation  integer     NOT NULL DEFAULT 1 CHECK (generation > 0),
    token_hash  bytea       NOT NULL UNIQUE CHECK (length(token_hash) = 32),
    expires_at  timestamptz,
    revoked_at  timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now(),
    rotated_at  timestamptz,
    UNIQUE (kind, resource_id)
);

CREATE TABLE guest_sessions (
    id            uuid        PRIMARY KEY DEFAULT uuidv7(),
    capability_id uuid        NOT NULL REFERENCES capabilities (id),
    generation    integer     NOT NULL CHECK (generation > 0),
    token_hash    bytea       NOT NULL UNIQUE CHECK (length(token_hash) = 32),
    created_at    timestamptz NOT NULL DEFAULT now(),
    expires_at    timestamptz NOT NULL,
    revoked_at    timestamptz
);
CREATE INDEX guest_sessions_active_by_capability ON guest_sessions (capability_id)
    WHERE revoked_at IS NULL;
CREATE INDEX guest_sessions_expiry ON guest_sessions (expires_at);

CREATE TABLE anonymous_sessions (
    id         uuid        PRIMARY KEY DEFAULT uuidv7(),
    token_hash bytea       NOT NULL UNIQUE CHECK (length(token_hash) = 32),
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL
);
CREATE INDEX anonymous_sessions_expiry ON anonymous_sessions (expires_at);

CREATE TABLE idempotency_requests (
    id           uuid        PRIMARY KEY DEFAULT uuidv7(),
    scope        text        NOT NULL CHECK (length(scope) BETWEEN 1 AND 200),
    operation    text        NOT NULL CHECK (length(operation) BETWEEN 1 AND 100),
    idem_key     uuid        NOT NULL,
    request_hash bytea       NOT NULL CHECK (length(request_hash) = 32),
    -- 0 only while the owning transaction is still running; never committed.
    status_code  integer     NOT NULL CHECK (status_code = 0 OR status_code BETWEEN 200 AND 599),
    -- AES-256-GCM sealed response body (nonce || ciphertext).
    response     bytea       NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    expires_at   timestamptz NOT NULL,
    UNIQUE (scope, operation, idem_key)
);
CREATE INDEX idempotency_requests_expiry ON idempotency_requests (expires_at);

-- +goose Down
-- Lossless only before real guest data exists; production recovery is a forward fix.
DROP TABLE idempotency_requests;
DROP TABLE anonymous_sessions;
DROP TABLE guest_sessions;
DROP TABLE capabilities;
