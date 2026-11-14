-- +goose Up

-- When a ticket entered its current status. The board orders a column by priority, then by
-- this time, so a card that just moved in sits below the ones that were there first.
ALTER TABLE tickets ADD COLUMN status_changed_at timestamptz;

-- Backfill from the last status change in the audit log, else the creation time. The backfill
-- is not an edit, so updated_at stays as it was.
ALTER TABLE tickets DISABLE TRIGGER tickets_updated_at;
UPDATE tickets t
SET status_changed_at = coalesce(
  (SELECT max(a.at) FROM audit_log a
   WHERE a.target_type = 'ticket' AND a.target_id = t.id AND a.action = 'ticket.status_changed'),
  t.created_at);
ALTER TABLE tickets ENABLE TRIGGER tickets_updated_at;

ALTER TABLE tickets ALTER COLUMN status_changed_at SET DEFAULT now(),
                    ALTER COLUMN status_changed_at SET NOT NULL;

-- One rule for every path that moves a ticket (manual moves, assignment, requester replies,
-- and the automatic moves later milestones add).
-- +goose StatementBegin
CREATE FUNCTION set_status_changed_at() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  NEW.status_changed_at = now();
  RETURN NEW;
END
$$;
-- +goose StatementEnd
CREATE TRIGGER tickets_status_changed_at BEFORE UPDATE OF status ON tickets
FOR EACH ROW WHEN (OLD.status IS DISTINCT FROM NEW.status) EXECUTE FUNCTION set_status_changed_at();

CREATE INDEX tickets_board_idx ON tickets (status, priority, status_changed_at);
CREATE INDEX tickets_resolved_at_idx ON tickets (resolved_at) WHERE resolved_at IS NOT NULL;

-- +goose Down
DROP INDEX tickets_resolved_at_idx;
DROP INDEX tickets_board_idx;
DROP TRIGGER tickets_status_changed_at ON tickets;
DROP FUNCTION set_status_changed_at();
ALTER TABLE tickets DROP COLUMN status_changed_at;
