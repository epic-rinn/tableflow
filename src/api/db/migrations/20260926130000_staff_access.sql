-- Staff identity and access (MVP-02): branches, staff accounts and roles,
-- single-use activation tokens, sessions, authentication throttling, audit.

-- +goose Up
CREATE TABLE branches (
    id         uuid        PRIMARY KEY DEFAULT uuidv7(),
    name       text        NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    timezone   text        NOT NULL DEFAULT 'Asia/Bangkok',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE staff_accounts (
    id            uuid        PRIMARY KEY DEFAULT uuidv7(),
    branch_id     uuid        NOT NULL REFERENCES branches (id),
    email         text        NOT NULL CHECK (email = lower(email) AND length(email) BETWEEN 3 AND 254),
    display_name  text        NOT NULL CHECK (length(display_name) BETWEEN 1 AND 100),
    password_hash text,
    status        text        NOT NULL CHECK (status IN ('invited', 'active', 'disabled')),
    version       integer     NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    CHECK (status <> 'active' OR password_hash IS NOT NULL),
    UNIQUE (branch_id, id)
);
CREATE UNIQUE INDEX staff_accounts_email_key ON staff_accounts (email);
CREATE INDEX staff_accounts_branch_page ON staff_accounts (branch_id, created_at, id);

CREATE TABLE staff_roles (
    staff_account_id uuid NOT NULL REFERENCES staff_accounts (id),
    role             text NOT NULL CHECK (role IN ('host', 'kitchen', 'cashier', 'manager')),
    PRIMARY KEY (staff_account_id, role)
);

CREATE TABLE staff_activation_tokens (
    id               uuid        PRIMARY KEY DEFAULT uuidv7(),
    staff_account_id uuid        NOT NULL REFERENCES staff_accounts (id),
    token_hash       bytea       NOT NULL UNIQUE CHECK (length(token_hash) = 32),
    expires_at       timestamptz NOT NULL,
    used_at          timestamptz,
    revoked_at       timestamptz,
    created_by       uuid        REFERENCES staff_accounts (id),
    created_at       timestamptz NOT NULL DEFAULT now()
);
-- At most one usable token per account; reissue revokes the previous one.
CREATE UNIQUE INDEX staff_activation_tokens_open ON staff_activation_tokens (staff_account_id)
    WHERE used_at IS NULL AND revoked_at IS NULL;

CREATE TABLE staff_sessions (
    id               uuid        PRIMARY KEY DEFAULT uuidv7(),
    staff_account_id uuid        NOT NULL REFERENCES staff_accounts (id),
    token_hash       bytea       NOT NULL UNIQUE CHECK (length(token_hash) = 32),
    created_at       timestamptz NOT NULL DEFAULT now(),
    last_seen_at     timestamptz NOT NULL DEFAULT now(),
    expires_at       timestamptz NOT NULL,
    revoked_at       timestamptz
);
CREATE INDEX staff_sessions_active_by_account ON staff_sessions (staff_account_id)
    WHERE revoked_at IS NULL;

CREATE TABLE auth_throttle (
    bucket       text        PRIMARY KEY CHECK (length(bucket) <= 200),
    window_start timestamptz NOT NULL,
    attempts     integer     NOT NULL CHECK (attempts >= 0)
);
CREATE INDEX auth_throttle_window ON auth_throttle (window_start);

CREATE TABLE audit_events (
    id            uuid        PRIMARY KEY DEFAULT uuidv7(),
    branch_id     uuid        NOT NULL REFERENCES branches (id),
    actor_staff_id uuid       REFERENCES staff_accounts (id),
    action        text        NOT NULL CHECK (length(action) BETWEEN 1 AND 100),
    resource_type text        NOT NULL,
    resource_id   uuid,
    reason        text        CHECK (reason IS NULL OR length(reason) <= 500),
    request_id    text        NOT NULL,
    details       jsonb       NOT NULL DEFAULT '{}'::jsonb,
    occurred_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_events_branch_time ON audit_events (branch_id, occurred_at, id);

-- +goose Down
-- Lossless only before real staff data exists; production recovery is a forward fix.
DROP TABLE audit_events;
DROP TABLE auth_throttle;
DROP TABLE staff_sessions;
DROP TABLE staff_activation_tokens;
DROP TABLE staff_roles;
DROP TABLE staff_accounts;
DROP TABLE branches;
