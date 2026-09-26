-- Member identity (MVP-04): accounts, sessions, verification/reset tokens.
-- Kept separate from staff credentials by design.

-- +goose Up
CREATE TABLE member_accounts (
    id                uuid        PRIMARY KEY DEFAULT uuidv7(),
    email             text        NOT NULL CHECK (email = lower(email) AND length(email) BETWEEN 3 AND 254),
    password_hash     text        NOT NULL,
    locale            text        NOT NULL CHECK (locale IN ('th', 'en')),
    email_verified_at timestamptz,
    version           integer     NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX member_accounts_email_key ON member_accounts (email);

CREATE TABLE member_sessions (
    id           uuid        PRIMARY KEY DEFAULT uuidv7(),
    member_id    uuid        NOT NULL REFERENCES member_accounts (id),
    token_hash   bytea       NOT NULL UNIQUE CHECK (length(token_hash) = 32),
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    expires_at   timestamptz NOT NULL,
    revoked_at   timestamptz
);
CREATE INDEX member_sessions_active_by_member ON member_sessions (member_id) WHERE revoked_at IS NULL;
CREATE INDEX member_sessions_expiry ON member_sessions (expires_at);

CREATE TABLE member_tokens (
    id         uuid        PRIMARY KEY DEFAULT uuidv7(),
    member_id  uuid        NOT NULL REFERENCES member_accounts (id),
    purpose    text        NOT NULL CHECK (purpose IN ('verify', 'reset')),
    token_hash bytea       NOT NULL UNIQUE CHECK (length(token_hash) = 32),
    expires_at timestamptz NOT NULL,
    used_at    timestamptz,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX member_tokens_open ON member_tokens (member_id, purpose)
    WHERE used_at IS NULL AND revoked_at IS NULL;
CREATE INDEX member_tokens_expiry ON member_tokens (expires_at);

-- +goose Down
-- Lossless only before real member data exists; production recovery is a forward fix.
DROP TABLE member_tokens;
DROP TABLE member_sessions;
DROP TABLE member_accounts;
