-- +goose Up

-- Definition of Done per project (HANDOFF §7). dod_config is internal/dod's Config JSON. The
-- column default has no conditions, so projects that existed before keep resolving by hand; new
-- projects get dod.Default() from the service. Changes apply to the project's open tickets.
ALTER TABLE projects
  ADD COLUMN dod_config jsonb NOT NULL DEFAULT
    '{"merged":{"enabled":false,"branch":"main"},"ci_passed":false,"deployed_production":false,"uat":{"enabled":false,"auto_approve_working_days":3},"requester_confirms":false}'
    CONSTRAINT projects_dod_config_object CHECK (jsonb_typeof(dod_config) = 'object'),
  ADD COLUMN allow_ticket_override boolean NOT NULL DEFAULT true;

-- A Definition of Done an agent set on one ticket, with the reason. A table of its own because
-- updating tickets would move the updated_at requesters see. Resetting to the project's deletes
-- the row.
CREATE TABLE ticket_dod_overrides (
  ticket_id uuid PRIMARY KEY CONSTRAINT ticket_dod_overrides_ticket_id_fkey REFERENCES tickets (id) ON DELETE CASCADE,
  config    jsonb NOT NULL CONSTRAINT ticket_dod_overrides_config_object CHECK (jsonb_typeof(config) = 'object'),
  reason    text NOT NULL CONSTRAINT ticket_dod_overrides_reason_length CHECK (char_length(reason) BETWEEN 1 AND 500),
  set_by    uuid NOT NULL CONSTRAINT ticket_dod_overrides_set_by_fkey REFERENCES users (id) ON DELETE RESTRICT,
  set_at    timestamptz NOT NULL DEFAULT now()
);

-- The Definition of Done items a ticket has met. done_by is the person who met it (the client
-- approving a test); NULL means the system (an overdue test approved automatically).
CREATE TABLE ticket_dod_items (
  ticket_id uuid NOT NULL CONSTRAINT ticket_dod_items_ticket_id_fkey REFERENCES tickets (id) ON DELETE CASCADE,
  item      text NOT NULL CONSTRAINT ticket_dod_items_item CHECK (item IN (
              'fix_committed', 'merged', 'ci_passed', 'deployed', 'uat_approved', 'requester_confirmed')),
  done_at   timestamptz NOT NULL DEFAULT now(),
  done_by   uuid CONSTRAINT ticket_dod_items_done_by_fkey REFERENCES users (id) ON DELETE RESTRICT,
  PRIMARY KEY (ticket_id, item)
);

-- +goose Down
DROP TABLE ticket_dod_items;
DROP TABLE ticket_dod_overrides;
ALTER TABLE projects DROP COLUMN allow_ticket_override, DROP COLUMN dod_config;
