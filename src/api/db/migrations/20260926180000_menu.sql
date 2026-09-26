-- Menu (MVP-08): categories → items → option groups → options, retired not deleted.

-- +goose Up
CREATE TABLE menus (
    branch_id  uuid        PRIMARY KEY REFERENCES branches (id),
    revision   integer     NOT NULL DEFAULT 1 CHECK (revision > 0),
    updated_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO menus (branch_id) SELECT id FROM branches;

CREATE TABLE menu_categories (
    id         uuid        PRIMARY KEY DEFAULT uuidv7(),
    branch_id  uuid        NOT NULL REFERENCES branches (id),
    name_th    text        NOT NULL CHECK (length(name_th) BETWEEN 1 AND 80),
    name_en    text        NOT NULL CHECK (length(name_en) BETWEEN 1 AND 80),
    sort       integer     NOT NULL,
    retired_at timestamptz,
    UNIQUE (branch_id, id)
);
CREATE INDEX menu_categories_active ON menu_categories (branch_id, sort) WHERE retired_at IS NULL;

CREATE TABLE menu_items (
    id               uuid        PRIMARY KEY DEFAULT uuidv7(),
    branch_id        uuid        NOT NULL REFERENCES branches (id),
    category_id      uuid        NOT NULL,
    name_th          text        NOT NULL CHECK (length(name_th) BETWEEN 1 AND 80),
    name_en          text        NOT NULL CHECK (length(name_en) BETWEEN 1 AND 80),
    price_satang     bigint      NOT NULL CHECK (price_satang BETWEEN 0 AND 10000000),
    sold_out         boolean     NOT NULL DEFAULT false,
    sort             integer     NOT NULL,
    -- Menu revision at which price/options/availability last changed; orders
    -- observed at an older revision are rejected for this item (ORD-002).
    changed_revision integer     NOT NULL CHECK (changed_revision > 0),
    version          integer     NOT NULL DEFAULT 1 CHECK (version > 0),
    retired_at       timestamptz,
    updated_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (branch_id, id),
    FOREIGN KEY (branch_id, category_id) REFERENCES menu_categories (branch_id, id)
);
CREATE INDEX menu_items_active ON menu_items (branch_id, category_id, sort) WHERE retired_at IS NULL;

CREATE TABLE option_groups (
    id          uuid        PRIMARY KEY DEFAULT uuidv7(),
    branch_id   uuid        NOT NULL,
    item_id     uuid        NOT NULL,
    name_th     text        NOT NULL CHECK (length(name_th) BETWEEN 1 AND 80),
    name_en     text        NOT NULL CHECK (length(name_en) BETWEEN 1 AND 80),
    min_choices integer     NOT NULL CHECK (min_choices >= 0),
    max_choices integer     NOT NULL CHECK (max_choices >= 1 AND max_choices >= min_choices AND max_choices <= 20),
    sort        integer     NOT NULL,
    retired_at  timestamptz,
    UNIQUE (branch_id, id),
    FOREIGN KEY (branch_id, item_id) REFERENCES menu_items (branch_id, id)
);
CREATE INDEX option_groups_by_item ON option_groups (item_id, sort) WHERE retired_at IS NULL;

CREATE TABLE menu_options (
    id                 uuid        PRIMARY KEY DEFAULT uuidv7(),
    branch_id          uuid        NOT NULL,
    group_id           uuid        NOT NULL,
    name_th            text        NOT NULL CHECK (length(name_th) BETWEEN 1 AND 80),
    name_en            text        NOT NULL CHECK (length(name_en) BETWEEN 1 AND 80),
    price_delta_satang bigint      NOT NULL CHECK (price_delta_satang BETWEEN 0 AND 10000000),
    sort               integer     NOT NULL,
    retired_at         timestamptz,
    FOREIGN KEY (branch_id, group_id) REFERENCES option_groups (branch_id, id)
);
CREATE INDEX menu_options_by_group ON menu_options (group_id, sort) WHERE retired_at IS NULL;

-- +goose Down
DROP TABLE menu_options;
DROP TABLE option_groups;
DROP TABLE menu_items;
DROP TABLE menu_categories;
DROP TABLE menus;
