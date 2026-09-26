-- Tables, seating groups and queue tickets (MVP-05).

-- +goose Up
CREATE TABLE seating_groups (
    id         uuid        PRIMARY KEY DEFAULT uuidv7(),
    branch_id  uuid        NOT NULL REFERENCES branches (id),
    label      text        NOT NULL CHECK (length(label) BETWEEN 1 AND 40),
    min_party  integer     NOT NULL CHECK (min_party >= 1),
    max_party  integer     NOT NULL CHECK (max_party BETWEEN min_party AND 50),
    retired_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (branch_id, id)
);
CREATE INDEX seating_groups_current ON seating_groups (branch_id, min_party) WHERE retired_at IS NULL;

CREATE TABLE dining_tables (
    id         uuid        PRIMARY KEY DEFAULT uuidv7(),
    branch_id  uuid        NOT NULL REFERENCES branches (id),
    label      text        NOT NULL CHECK (length(label) BETWEEN 1 AND 20),
    capacity   integer     NOT NULL CHECK (capacity BETWEEN 1 AND 50),
    needs      text[]      NOT NULL DEFAULT '{}' CHECK (needs <@ ARRAY['accessible', 'high_chair']::text[]),
    state      text        NOT NULL DEFAULT 'available' CHECK (state IN ('available', 'held', 'occupied', 'cleaning')),
    active     boolean     NOT NULL DEFAULT true,
    version    integer     NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (branch_id, label),
    UNIQUE (branch_id, id)
);

CREATE TABLE queue_counters (
    branch_id     uuid    NOT NULL REFERENCES branches (id),
    business_date date    NOT NULL,
    last_number   integer NOT NULL CHECK (last_number > 0),
    PRIMARY KEY (branch_id, business_date)
);

CREATE TABLE queue_tickets (
    id               uuid        PRIMARY KEY DEFAULT uuidv7(),
    branch_id        uuid        NOT NULL REFERENCES branches (id),
    business_date    date        NOT NULL,
    display_number   integer     NOT NULL CHECK (display_number > 0),
    -- Priority across days; never reset (QUE-001, QUE-A4).
    join_order       bigint      GENERATED ALWAYS AS IDENTITY UNIQUE,
    party_size       integer     NOT NULL CHECK (party_size BETWEEN 1 AND 50),
    needs            text[]      NOT NULL DEFAULT '{}' CHECK (needs <@ ARRAY['accessible', 'high_chair']::text[]),
    seating_group_id uuid,
    source           text        NOT NULL CHECK (source IN ('guest', 'staff')),
    state            text        NOT NULL DEFAULT 'waiting' CHECK (state IN ('waiting', 'called', 'seated', 'cancelled', 'no_show')),
    called_table_id  uuid,
    called_at        timestamptz,
    called_until     timestamptz,
    terminal_at      timestamptz,
    version          integer     NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (branch_id, business_date, display_number),
    UNIQUE (branch_id, id),
    FOREIGN KEY (branch_id, seating_group_id) REFERENCES seating_groups (branch_id, id),
    FOREIGN KEY (branch_id, called_table_id) REFERENCES dining_tables (branch_id, id),
    CHECK ((state = 'called') = (called_table_id IS NOT NULL AND called_until IS NOT NULL))
);
-- Active board and position queries never scan historical tickets.
CREATE INDEX queue_tickets_active ON queue_tickets (branch_id, join_order) WHERE state IN ('waiting', 'called');
CREATE INDEX queue_tickets_waiting_group ON queue_tickets (branch_id, seating_group_id, join_order) WHERE state = 'waiting';

-- Default bands (pilot defaults, editable by managers).
INSERT INTO seating_groups (branch_id, label, min_party, max_party)
SELECT b.id, g.label, g.lo, g.hi
FROM branches b
CROSS JOIN (VALUES ('1–2', 1, 2), ('3–4', 3, 4), ('5–6', 5, 6)) AS g (label, lo, hi);

-- +goose Down
DROP TABLE queue_tickets;
DROP TABLE queue_counters;
DROP TABLE dining_tables;
DROP TABLE seating_groups;
