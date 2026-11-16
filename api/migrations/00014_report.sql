-- +goose Up

-- SLA report (M8): a report month holds the tickets created in it.
CREATE INDEX tickets_created_at_idx ON tickets (created_at);

-- +goose Down
DROP INDEX tickets_created_at_idx;
