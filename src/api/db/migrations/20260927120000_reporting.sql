-- Business-date reporting (MVP-16): time-range indexes per branch. Each
-- backs one aggregate in the daily report (see 016-reporting plans).

-- +goose Up
CREATE INDEX queue_tickets_by_created ON queue_tickets (branch_id, created_at);
CREATE INDEX visits_by_opened ON visits (branch_id, opened_at);
CREATE INDEX refunds_by_created ON refunds (branch_id, created_at);
CREATE INDEX loyalty_ledger_by_branch ON loyalty_ledger (branch_id, created_at);

-- +goose Down
DROP INDEX loyalty_ledger_by_branch;
DROP INDEX refunds_by_created;
DROP INDEX visits_by_opened;
DROP INDEX queue_tickets_by_created;
