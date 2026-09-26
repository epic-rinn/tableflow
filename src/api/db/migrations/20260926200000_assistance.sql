-- Assistance requests (MVP-10).

-- +goose Up
CREATE TABLE assistance_requests (
    id              uuid        PRIMARY KEY DEFAULT uuidv7(),
    branch_id       uuid        NOT NULL,
    visit_id        uuid        NOT NULL,
    topic           text        NOT NULL CHECK (topic IN ('help', 'allergy', 'checkout')),
    note            text        CHECK (note IS NULL OR length(note) <= 500),
    state           text        NOT NULL DEFAULT 'open' CHECK (state IN ('open', 'acknowledged', 'resolved')),
    raised_by       text        NOT NULL CHECK (raised_by IN ('guest', 'staff')),
    acknowledged_by uuid        REFERENCES staff_accounts (id),
    acknowledged_at timestamptz,
    resolved_at     timestamptz,
    version         integer     NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at      timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (branch_id, visit_id) REFERENCES visits (branch_id, id)
);
-- One outstanding request per visit and topic (ORD-006).
CREATE UNIQUE INDEX assistance_outstanding ON assistance_requests (visit_id, topic) WHERE state IN ('open', 'acknowledged');
CREATE INDEX assistance_board ON assistance_requests (branch_id, created_at, id) WHERE state IN ('open', 'acknowledged');

-- +goose Down
DROP TABLE assistance_requests;
