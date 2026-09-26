-- Orders with immutable line snapshots (MVP-09).

-- +goose Up
ALTER TABLE visits ADD COLUMN bill_version integer NOT NULL DEFAULT 1 CHECK (bill_version > 0);

CREATE TABLE orders (
    id                     uuid        PRIMARY KEY DEFAULT uuidv7(),
    branch_id              uuid        NOT NULL,
    visit_id               uuid        NOT NULL,
    actor_kind             text        NOT NULL CHECK (actor_kind IN ('guest', 'staff')),
    actor_staff_id         uuid        REFERENCES staff_accounts (id),
    actor_guest_session_id uuid,
    menu_revision          integer     NOT NULL,
    created_at             timestamptz NOT NULL DEFAULT now(),
    UNIQUE (branch_id, id),
    FOREIGN KEY (branch_id, visit_id) REFERENCES visits (branch_id, id),
    CHECK ((actor_kind = 'staff') = (actor_staff_id IS NOT NULL))
);
CREATE INDEX orders_by_visit ON orders (visit_id, created_at, id);

CREATE TABLE order_lines (
    id                uuid        PRIMARY KEY DEFAULT uuidv7(),
    branch_id         uuid        NOT NULL,
    order_id          uuid        NOT NULL,
    visit_id          uuid        NOT NULL,
    -- Snapshot: menu rows may later change or retire; charges never follow them.
    item_id           uuid        NOT NULL,
    name_th           text        NOT NULL,
    name_en           text        NOT NULL,
    options           jsonb       NOT NULL DEFAULT '[]'::jsonb,
    unit_price_satang bigint      NOT NULL CHECK (unit_price_satang >= 0),
    quantity          integer     NOT NULL CHECK (quantity BETWEEN 1 AND 20),
    note              text        CHECK (note IS NULL OR length(note) <= 500),
    state             text        NOT NULL DEFAULT 'submitted'
                      CHECK (state IN ('submitted', 'accepted', 'preparing', 'ready', 'served', 'rejected', 'cancelled')),
    reason            text        CHECK (reason IS NULL OR length(reason) <= 500),
    version           integer     NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (branch_id, order_id) REFERENCES orders (branch_id, id),
    FOREIGN KEY (branch_id, visit_id) REFERENCES visits (branch_id, id),
    CHECK ((state IN ('rejected', 'cancelled')) = (reason IS NOT NULL))
);
CREATE INDEX order_lines_by_order ON order_lines (order_id, created_at, id);
CREATE INDEX order_lines_by_visit ON order_lines (visit_id);
-- Kitchen board: active lines only, never historical ones.
CREATE INDEX order_lines_kitchen ON order_lines (branch_id, created_at, id)
    WHERE state IN ('submitted', 'accepted', 'preparing', 'ready');

-- +goose Down
DROP TABLE order_lines;
DROP TABLE orders;
ALTER TABLE visits DROP COLUMN bill_version;
