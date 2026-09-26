-- Holds, visits and exclusive table claims (MVP-06).

-- +goose Up
ALTER TABLE branches ADD COLUMN call_hold_minutes integer NOT NULL DEFAULT 5 CHECK (call_hold_minutes BETWEEN 1 AND 60);

CREATE TABLE visits (
    id              uuid        PRIMARY KEY DEFAULT uuidv7(),
    branch_id       uuid        NOT NULL REFERENCES branches (id),
    table_id        uuid        NOT NULL,
    queue_ticket_id uuid        UNIQUE,
    party_size      integer     NOT NULL CHECK (party_size BETWEEN 1 AND 50),
    needs           text[]      NOT NULL DEFAULT '{}',
    state           text        NOT NULL DEFAULT 'open' CHECK (state IN ('open', 'paid', 'departed', 'closed')),
    close_reason    text        CHECK (close_reason IS NULL OR length(close_reason) <= 500),
    opened_at       timestamptz NOT NULL DEFAULT now(),
    paid_at         timestamptz,
    ended_at        timestamptz,
    version         integer     NOT NULL DEFAULT 1 CHECK (version > 0),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (branch_id, id),
    FOREIGN KEY (branch_id, table_id) REFERENCES dining_tables (branch_id, id),
    FOREIGN KEY (branch_id, queue_ticket_id) REFERENCES queue_tickets (branch_id, id),
    CHECK ((state IN ('departed', 'closed')) = (ended_at IS NOT NULL))
);
CREATE INDEX visits_active ON visits (branch_id, opened_at, id) WHERE state IN ('open', 'paid');

-- The single exclusivity mechanism for holds and visits (SEA-002).
CREATE TABLE table_claims (
    table_id        uuid        PRIMARY KEY,
    branch_id       uuid        NOT NULL,
    queue_ticket_id uuid        UNIQUE,
    visit_id        uuid        UNIQUE,
    created_at      timestamptz NOT NULL DEFAULT now(),
    CHECK (num_nonnulls(queue_ticket_id, visit_id) = 1),
    FOREIGN KEY (branch_id, table_id) REFERENCES dining_tables (branch_id, id),
    FOREIGN KEY (branch_id, queue_ticket_id) REFERENCES queue_tickets (branch_id, id),
    FOREIGN KEY (branch_id, visit_id) REFERENCES visits (branch_id, id)
);

-- +goose Down
DROP TABLE table_claims;
DROP TABLE visits;
ALTER TABLE branches DROP COLUMN call_hold_minutes;
