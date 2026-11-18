-- +goose Up

-- The hourly auth purge (M10 security review) deletes old sign-in attempts by time alone; the
-- (key, at) index can't serve that.
CREATE INDEX login_attempts_at_idx ON login_attempts (at);

-- +goose Down

DROP INDEX login_attempts_at_idx;
