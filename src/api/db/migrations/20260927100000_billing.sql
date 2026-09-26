-- Charge policies, frozen bill snapshots, settlements and full refunds (MVP-11/12/13).

-- +goose Up
ALTER TABLE visits DROP CONSTRAINT visits_state_check;
ALTER TABLE visits ADD CONSTRAINT visits_state_check
    CHECK (state IN ('open', 'settling', 'paid', 'departed', 'closed'));
DROP INDEX visits_active;
CREATE INDEX visits_active ON visits (branch_id, opened_at, id) WHERE state IN ('open', 'settling', 'paid');

-- Append-only; the highest version is current. No statutory defaults: a
-- branch without rows is "unconfigured" (0% / exclusive) until a manager
-- enters operator-validated rates (BIL-004).
CREATE TABLE charge_policies (
    branch_id   uuid        NOT NULL REFERENCES branches (id),
    version     integer     NOT NULL CHECK (version > 0),
    tax_mode    text        NOT NULL CHECK (tax_mode IN ('exclusive', 'inclusive')),
    tax_bp      integer     NOT NULL CHECK (tax_bp BETWEEN 0 AND 10000),
    service_bp  integer     NOT NULL CHECK (service_bp BETWEEN 0 AND 10000),
    created_by  uuid        NOT NULL REFERENCES staff_accounts (id),
    created_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (branch_id, version)
);

-- Totals frozen when settlement begins; one per (visit, bill version).
CREATE TABLE bill_snapshots (
    id              uuid        PRIMARY KEY DEFAULT uuidv7(),
    branch_id       uuid        NOT NULL,
    visit_id        uuid        NOT NULL,
    bill_version    integer     NOT NULL CHECK (bill_version > 0),
    policy_version  integer     NOT NULL CHECK (policy_version >= 0),
    tax_mode        text        NOT NULL CHECK (tax_mode IN ('exclusive', 'inclusive')),
    tax_bp          integer     NOT NULL CHECK (tax_bp BETWEEN 0 AND 10000),
    service_bp      integer     NOT NULL CHECK (service_bp BETWEEN 0 AND 10000),
    discount_bp     integer     NOT NULL CHECK (discount_bp BETWEEN 0 AND 10000),
    gross_satang    bigint      NOT NULL CHECK (gross_satang >= 0),
    discount_satang bigint      NOT NULL CHECK (discount_satang >= 0),
    service_satang  bigint      NOT NULL CHECK (service_satang >= 0),
    tax_satang      bigint      NOT NULL CHECK (tax_satang >= 0),
    total_satang    bigint      NOT NULL CHECK (total_satang >= 0),
    lines           jsonb       NOT NULL,
    created_by      uuid        NOT NULL REFERENCES staff_accounts (id),
    created_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (branch_id, id),
    UNIQUE (visit_id, bill_version),
    FOREIGN KEY (branch_id, visit_id) REFERENCES visits (branch_id, id)
);

-- One settlement per visit; immutable once written (BIL-006/008).
CREATE TABLE settlements (
    id                 uuid        PRIMARY KEY DEFAULT uuidv7(),
    branch_id          uuid        NOT NULL,
    visit_id           uuid        NOT NULL UNIQUE,
    snapshot_id        uuid        NOT NULL,
    receipt_reference  text        NOT NULL UNIQUE CHECK (receipt_reference ~ '^R-[A-Z2-7]{10}$'),
    amount_satang      bigint      NOT NULL CHECK (amount_satang >= 0),
    method             text        NOT NULL CHECK (method IN ('cash', 'bank_transfer', 'card', 'other')),
    verification_note  text        NOT NULL CHECK (length(verification_note) BETWEEN 1 AND 500),
    external_reference text        CHECK (external_reference IS NULL OR length(external_reference) BETWEEN 1 AND 200),
    confirmed_by       uuid        NOT NULL REFERENCES staff_accounts (id),
    paid_at            timestamptz NOT NULL DEFAULT now(),
    UNIQUE (branch_id, id),
    FOREIGN KEY (branch_id, visit_id) REFERENCES visits (branch_id, id),
    FOREIGN KEY (branch_id, snapshot_id) REFERENCES bill_snapshots (branch_id, id)
);
CREATE INDEX settlements_by_branch ON settlements (branch_id, paid_at DESC, id DESC);

-- Full refunds only; money moves outside the system (BIL-007).
CREATE TABLE refunds (
    id                 uuid        PRIMARY KEY DEFAULT uuidv7(),
    branch_id          uuid        NOT NULL,
    settlement_id      uuid        NOT NULL UNIQUE,
    amount_satang      bigint      NOT NULL CHECK (amount_satang >= 0),
    reason             text        NOT NULL CHECK (length(reason) BETWEEN 1 AND 500),
    external_reference text        NOT NULL CHECK (length(external_reference) BETWEEN 1 AND 200),
    recorded_by        uuid        NOT NULL REFERENCES staff_accounts (id),
    created_at         timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (branch_id, settlement_id) REFERENCES settlements (branch_id, id)
);

-- +goose Down
DROP TABLE refunds;
DROP TABLE settlements;
DROP TABLE bill_snapshots;
DROP TABLE charge_policies;
DROP INDEX visits_active;
CREATE INDEX visits_active ON visits (branch_id, opened_at, id) WHERE state IN ('open', 'paid');
ALTER TABLE visits DROP CONSTRAINT visits_state_check;
ALTER TABLE visits ADD CONSTRAINT visits_state_check CHECK (state IN ('open', 'paid', 'departed', 'closed'));
