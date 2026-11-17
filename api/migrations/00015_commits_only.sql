-- +goose Up

-- Owner, 17 Nov: Git no longer moves tickets or ticks their Definition of Done, and the
-- connector reads commits only (no pull/merge requests, CI runs or deployments). Audit entries
-- and notifications written before keep their old values: they are history and can't change.

-- A Definition of Done keeps the client's checks. Projects with only code conditions now
-- resolve by hand.
UPDATE projects SET dod_config = dod_config - 'merged' - 'ci_passed' - 'deployed_production';
UPDATE ticket_dod_overrides SET config = config - 'merged' - 'ci_passed' - 'deployed_production';
ALTER TABLE projects ALTER COLUMN dod_config
  SET DEFAULT '{"uat":{"enabled":false,"auto_approve_working_days":3},"requester_confirms":false}';
DELETE FROM ticket_dod_items WHERE item NOT IN ('uat_approved', 'requester_confirmed');
ALTER TABLE ticket_dod_items DROP CONSTRAINT ticket_dod_items_item,
  ADD CONSTRAINT ticket_dod_items_item CHECK (item IN ('uat_approved', 'requester_confirmed')),
  DROP COLUMN code_event_id;

-- Commits only. Links of other events go with them (ON DELETE CASCADE).
DELETE FROM code_events WHERE kind <> 'commit';
DROP INDEX code_events_runs_idx;
ALTER TABLE code_events DROP CONSTRAINT code_events_kind,
  ADD CONSTRAINT code_events_kind CHECK (kind = 'commit'),
  DROP COLUMN number,
  DROP COLUMN target,
  -- Every branch the commit was pushed to, in the order they were seen: the fix branch, then
  -- main once it merges. branch stays the first one.
  ADD COLUMN branches text[] NOT NULL DEFAULT '{}';
UPDATE code_events SET branches = ARRAY[branch] WHERE branch <> '';

-- A link only says whether the commit reached the ticket.
UPDATE code_event_links SET outcome = 'linked'
  WHERE outcome NOT IN ('linked', 'ticket_closed', 'unknown_ticket', 'other_repository');
ALTER TABLE code_event_links DROP CONSTRAINT code_event_links_status_pair,
  DROP CONSTRAINT code_event_links_outcome,
  ADD CONSTRAINT code_event_links_outcome CHECK (outcome IN ('linked', 'ticket_closed', 'unknown_ticket', 'other_repository')),
  DROP COLUMN items,
  DROP COLUMN status_from,
  DROP COLUMN status_to;

-- +goose Down
-- The columns and values come back; the deleted events, items and conditions don't.
ALTER TABLE code_event_links ADD COLUMN items text[] NOT NULL DEFAULT '{}',
  ADD COLUMN status_from text,
  ADD COLUMN status_to text,
  DROP CONSTRAINT code_event_links_outcome,
  ADD CONSTRAINT code_event_links_outcome CHECK (outcome IN (
    'linked', 'in_review', 'fix_committed', 'merged', 'ci_passed', 'deployed', 'reopened',
    'ticket_closed', 'unknown_ticket', 'other_repository')),
  ADD CONSTRAINT code_event_links_status_pair CHECK ((status_from IS NULL) = (status_to IS NULL));
ALTER TABLE code_events DROP COLUMN branches,
  ADD COLUMN number bigint,
  ADD COLUMN target text NOT NULL DEFAULT '' CONSTRAINT code_events_target_length CHECK (char_length(target) <= 255),
  DROP CONSTRAINT code_events_kind,
  ADD CONSTRAINT code_events_kind CHECK (kind IN ('commit', 'pr_opened', 'pr_merged', 'pr_closed', 'pipeline', 'deployment'));
CREATE INDEX code_events_runs_idx ON code_events (repository_id, occurred_at) WHERE kind IN ('pipeline', 'deployment');
ALTER TABLE ticket_dod_items DROP CONSTRAINT ticket_dod_items_item,
  ADD CONSTRAINT ticket_dod_items_item CHECK (item IN (
    'fix_committed', 'merged', 'ci_passed', 'deployed', 'uat_approved', 'requester_confirmed')),
  ADD COLUMN code_event_id uuid CONSTRAINT ticket_dod_items_code_event_id_fkey REFERENCES code_events (id) ON DELETE SET NULL;
ALTER TABLE projects ALTER COLUMN dod_config SET DEFAULT
  '{"merged":{"enabled":false,"branch":"main"},"ci_passed":false,"deployed_production":false,"uat":{"enabled":false,"auto_approve_working_days":3},"requester_confirms":false}';
