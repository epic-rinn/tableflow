-- Baseline: assert the supported PostgreSQL major before any schema exists.
-- Domain tables are added by their feature changes, not speculatively here.

-- +goose Up
-- +goose StatementBegin
DO $$
BEGIN
    IF current_setting('server_version_num')::integer < 180000 THEN
        RAISE EXCEPTION 'TableFlow requires PostgreSQL 18 or later (found %)',
            current_setting('server_version');
    END IF;
END
$$;
-- +goose StatementEnd

-- +goose Down
-- Nothing to reverse: the Up section creates no objects.
SELECT 1;
