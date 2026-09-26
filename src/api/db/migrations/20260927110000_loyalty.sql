-- Member visit claims, loyalty policies, member profiles and the loyalty
-- ledger (MVP-14/15).

-- +goose Up
-- Append-only; the highest version is current. Version 0 (no rows) means the
-- documented pilot defaults (specs/product/mvp.md), not confirmed economics.
CREATE TABLE loyalty_policies (
    branch_id          uuid        NOT NULL REFERENCES branches (id),
    version            integer     NOT NULL CHECK (version > 0),
    satang_per_point   integer     NOT NULL CHECK (satang_per_point BETWEEN 100 AND 10000000),
    silver_threshold   bigint      NOT NULL CHECK (silver_threshold > 0),
    silver_discount_bp integer     NOT NULL CHECK (silver_discount_bp BETWEEN 0 AND 5000),
    gold_threshold     bigint      NOT NULL CHECK (gold_threshold > 0),
    gold_discount_bp   integer     NOT NULL CHECK (gold_discount_bp BETWEEN 0 AND 5000),
    created_by         uuid        NOT NULL REFERENCES staff_accounts (id),
    created_at         timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (branch_id, version),
    CHECK (gold_threshold > silver_threshold)
);

-- Per-branch loyalty state of a member; totals always equal the ledger sums.
CREATE TABLE member_profiles (
    id                      uuid        PRIMARY KEY DEFAULT uuidv7(),
    branch_id               uuid        NOT NULL REFERENCES branches (id),
    member_id               uuid        NOT NULL REFERENCES member_accounts (id),
    points_balance          bigint      NOT NULL DEFAULT 0 CHECK (points_balance >= 0),
    qualifying_spend_satang bigint      NOT NULL DEFAULT 0 CHECK (qualifying_spend_satang >= 0),
    tier                    text        NOT NULL DEFAULT 'base' CHECK (tier IN ('base', 'silver', 'gold')),
    version                 integer     NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at              timestamptz NOT NULL DEFAULT now(),
    updated_at              timestamptz NOT NULL DEFAULT now(),
    UNIQUE (branch_id, member_id),
    UNIQUE (branch_id, id)
);

ALTER TABLE visits
    ADD COLUMN member_id         uuid REFERENCES member_accounts (id),
    ADD COLUMN member_claimed_at timestamptz,
    ADD CONSTRAINT visits_claim_consistent CHECK ((member_id IS NULL) = (member_claimed_at IS NULL));
CREATE INDEX visits_by_member ON visits (member_id) WHERE member_id IS NOT NULL;

-- The benefit frozen at begin-settlement (LOY-003).
ALTER TABLE bill_snapshots
    ADD COLUMN member_id              uuid REFERENCES member_accounts (id),
    ADD COLUMN member_tier            text CHECK (member_tier IN ('base', 'silver', 'gold')),
    ADD COLUMN loyalty_policy_version integer CHECK (loyalty_policy_version >= 0),
    ADD COLUMN satang_per_point       integer CHECK (satang_per_point > 0),
    ADD CONSTRAINT bill_snapshots_member_consistent
        CHECK ((member_id IS NULL) = (member_tier IS NULL)
           AND (member_id IS NULL) = (loyalty_policy_version IS NULL)
           AND (member_id IS NULL) = (satang_per_point IS NULL));

-- Applied policy and eligible amount stored with the settlement (LOY-004).
ALTER TABLE settlements
    ADD COLUMN member_id              uuid REFERENCES member_accounts (id),
    ADD COLUMN eligible_satang        bigint CHECK (eligible_satang >= 0),
    ADD COLUMN points_earned          bigint CHECK (points_earned >= 0),
    ADD COLUMN loyalty_policy_version integer CHECK (loyalty_policy_version >= 0),
    ADD CONSTRAINT settlements_member_consistent
        CHECK ((member_id IS NULL) = (eligible_satang IS NULL)
           AND (member_id IS NULL) = (points_earned IS NULL)
           AND (member_id IS NULL) = (loyalty_policy_version IS NULL));

-- Append-only; never edited or deleted (LOY-005/006).
CREATE TABLE loyalty_ledger (
    id                      uuid        PRIMARY KEY DEFAULT uuidv7(),
    branch_id               uuid        NOT NULL,
    member_id               uuid        NOT NULL REFERENCES member_accounts (id),
    settlement_id           uuid        NOT NULL,
    kind                    text        NOT NULL CHECK (kind IN ('earn', 'reversal')),
    points_delta            bigint      NOT NULL,
    qualifying_delta_satang bigint      NOT NULL,
    policy_version          integer     NOT NULL CHECK (policy_version >= 0),
    created_at              timestamptz NOT NULL DEFAULT now(),
    UNIQUE (settlement_id, kind),
    FOREIGN KEY (branch_id, settlement_id) REFERENCES settlements (branch_id, id),
    CHECK (CASE kind WHEN 'earn' THEN points_delta >= 0 AND qualifying_delta_satang >= 0
                     ELSE points_delta <= 0 AND qualifying_delta_satang <= 0 END)
);
CREATE INDEX loyalty_ledger_by_member ON loyalty_ledger (member_id, created_at DESC, id DESC);

-- +goose Down
DROP TABLE loyalty_ledger;
ALTER TABLE settlements DROP CONSTRAINT settlements_member_consistent,
    DROP COLUMN member_id, DROP COLUMN eligible_satang, DROP COLUMN points_earned, DROP COLUMN loyalty_policy_version;
ALTER TABLE bill_snapshots DROP CONSTRAINT bill_snapshots_member_consistent,
    DROP COLUMN member_id, DROP COLUMN member_tier, DROP COLUMN loyalty_policy_version, DROP COLUMN satang_per_point;
DROP INDEX visits_by_member;
ALTER TABLE visits DROP CONSTRAINT visits_claim_consistent, DROP COLUMN member_id, DROP COLUMN member_claimed_at;
DROP TABLE member_profiles;
DROP TABLE loyalty_policies;
