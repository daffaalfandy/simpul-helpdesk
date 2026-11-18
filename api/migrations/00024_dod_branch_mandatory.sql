-- +goose Up

-- 10b (owner 18 Nov): a project's Definition of Done can be guidance only. Mandatory keeps the
-- rule so far: resolving with it unmet needs a reason, recorded as dod_skipped, and supervisors
-- are told. Every project, existing or new, starts mandatory; a ticket's own Definition of Done
-- can't change it.
ALTER TABLE projects ADD COLUMN dod_mandatory boolean NOT NULL DEFAULT true;

-- "Code reached a branch": Git ticks this item when a commit naming the ticket reaches the
-- project's branch, and code_event_id names that commit. Only this item has one.
ALTER TABLE ticket_dod_items DROP CONSTRAINT ticket_dod_items_item,
  ADD CONSTRAINT ticket_dod_items_item CHECK (item IN ('code_on_branch', 'uat_approved', 'requester_confirmed')),
  ADD COLUMN code_event_id uuid
    CONSTRAINT ticket_dod_items_code_event_id_fkey REFERENCES code_events (id) ON DELETE SET NULL,
  ADD CONSTRAINT ticket_dod_items_code_event CHECK (code_event_id IS NULL OR item = 'code_on_branch');

-- +goose Down
DELETE FROM ticket_dod_items WHERE item = 'code_on_branch';
ALTER TABLE ticket_dod_items DROP CONSTRAINT ticket_dod_items_code_event,
  DROP COLUMN code_event_id,
  DROP CONSTRAINT ticket_dod_items_item,
  ADD CONSTRAINT ticket_dod_items_item CHECK (item IN ('uat_approved', 'requester_confirmed'));
UPDATE projects SET dod_config = dod_config - 'code_on_branch';
UPDATE ticket_dod_overrides SET config = config - 'code_on_branch';
ALTER TABLE projects DROP COLUMN dod_mandatory;
