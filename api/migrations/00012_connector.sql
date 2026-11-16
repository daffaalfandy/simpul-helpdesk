-- +goose Up

-- GitHub/GitLab connector (HANDOFF §8), read-only through webhooks: each repository has its own
-- webhook secret, sealed with ENCRYPTION_KEY (internal/secretbox) and bound to the row id.
-- external_id is the provider's own id, learnt from the first verified delivery, so a renamed
-- repository is still recognised. Repositories with code events are hidden, never deleted.
CREATE TABLE repositories (
  id               uuid PRIMARY KEY DEFAULT uuidv7(),
  provider         text NOT NULL CONSTRAINT repositories_provider CHECK (provider IN ('github', 'gitlab')),
  host             text NOT NULL CONSTRAINT repositories_host_length CHECK (char_length(host) BETWEEN 1 AND 253),
  full_name        text NOT NULL CONSTRAINT repositories_full_name_length CHECK (char_length(full_name) BETWEEN 3 AND 255),
  web_url          text NOT NULL CONSTRAINT repositories_web_url_length CHECK (char_length(web_url) BETWEEN 1 AND 600),
  external_id      text CONSTRAINT repositories_external_id_length CHECK (char_length(external_id) BETWEEN 1 AND 100),
  webhook_secret   bytea NOT NULL,
  hidden           boolean NOT NULL DEFAULT false,
  last_delivery_at timestamptz,
  last_error       text CONSTRAINT repositories_last_error CHECK (last_error IN (
                     'bad_signature', 'repository_mismatch', 'malformed', 'too_large')),
  last_error_at    timestamptz,
  created_by       uuid NOT NULL CONSTRAINT repositories_created_by_fkey REFERENCES users (id) ON DELETE RESTRICT,
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT repositories_error_time CHECK ((last_error IS NULL) = (last_error_at IS NULL))
);
CREATE UNIQUE INDEX repositories_name_key ON repositories (provider, lower(host), lower(full_name));
CREATE TRIGGER repositories_updated_at BEFORE UPDATE ON repositories
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- A project reads code from at most one repository; one repository can serve several projects
-- (the same system run for several clients).
ALTER TABLE projects ADD COLUMN repository_id uuid
  CONSTRAINT projects_repository_id_fkey REFERENCES repositories (id) ON DELETE SET NULL;
CREATE INDEX projects_repository_id_idx ON projects (repository_id) WHERE repository_id IS NOT NULL;

-- Verified deliveries as received, for the processing job and redelivery checks. Payloads carry
-- author names and emails (UU PDP), so a daily job deletes them after 30 days; code_events keep
-- what the helpdesk needs.
CREATE TABLE webhook_deliveries (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  repository_id uuid NOT NULL CONSTRAINT webhook_deliveries_repository_id_fkey REFERENCES repositories (id) ON DELETE CASCADE,
  delivery_id   text NOT NULL CONSTRAINT webhook_deliveries_delivery_id_length CHECK (char_length(delivery_id) BETWEEN 1 AND 200),
  event_name    text NOT NULL CONSTRAINT webhook_deliveries_event_name_length CHECK (char_length(event_name) BETWEEN 1 AND 100),
  payload       jsonb NOT NULL,
  received_at   timestamptz NOT NULL DEFAULT now(),
  processed_at  timestamptz,
  CONSTRAINT webhook_deliveries_delivery_key UNIQUE (repository_id, delivery_id)
);
CREATE INDEX webhook_deliveries_received_at_idx ON webhook_deliveries (received_at);

-- What happened in a repository, one row per fact: a commit naming a ticket, a pull/merge
-- request opened, merged or closed, a successful CI run, a production deployment. Unique per
-- repository, kind and the provider's id, so a redelivery adds nothing. CI runs and deployments
-- are kept even when they match no ticket yet: a merge read later still finds them.
CREATE TABLE code_events (
  id             uuid PRIMARY KEY DEFAULT uuidv7(),
  repository_id  uuid NOT NULL CONSTRAINT code_events_repository_id_fkey REFERENCES repositories (id) ON DELETE RESTRICT,
  kind           text NOT NULL CONSTRAINT code_events_kind CHECK (kind IN (
                   'commit', 'pr_opened', 'pr_merged', 'pr_closed', 'pipeline', 'deployment')),
  external_id    text NOT NULL CONSTRAINT code_events_external_id_length CHECK (char_length(external_id) BETWEEN 1 AND 200),
  sha            text NOT NULL DEFAULT '' CONSTRAINT code_events_sha_length CHECK (char_length(sha) <= 64),
  number         bigint,
  title          text NOT NULL DEFAULT '' CONSTRAINT code_events_title_length CHECK (char_length(title) <= 200),
  branch         text NOT NULL DEFAULT '' CONSTRAINT code_events_branch_length CHECK (char_length(branch) <= 255),
  target         text NOT NULL DEFAULT '' CONSTRAINT code_events_target_length CHECK (char_length(target) <= 255),
  url            text NOT NULL DEFAULT '' CONSTRAINT code_events_url_length CHECK (char_length(url) <= 2000),
  author_name    text NOT NULL DEFAULT '' CONSTRAINT code_events_author_name_length CHECK (char_length(author_name) <= 200),
  author_login   text NOT NULL DEFAULT '' CONSTRAINT code_events_author_login_length CHECK (char_length(author_login) <= 200),
  author_user_id uuid CONSTRAINT code_events_author_user_id_fkey REFERENCES users (id) ON DELETE SET NULL,
  delivery_id    uuid CONSTRAINT code_events_delivery_id_fkey REFERENCES webhook_deliveries (id) ON DELETE SET NULL,
  occurred_at    timestamptz NOT NULL,
  created_at     timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT code_events_external_key UNIQUE (repository_id, kind, external_id)
);
CREATE INDEX code_events_runs_idx ON code_events (repository_id, occurred_at) WHERE kind IN ('pipeline', 'deployment');
CREATE INDEX code_events_delivery_id_idx ON code_events (delivery_id) WHERE delivery_id IS NOT NULL;

-- What an event did to a ticket: one row each time it changed something, plus the keys it named
-- that were ignored (no such ticket, another repository's ticket, a closed ticket). Feeds the
-- recent-events log, the ticket's code activity and the board's code chip.
CREATE TABLE code_event_links (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  code_event_id uuid NOT NULL CONSTRAINT code_event_links_code_event_id_fkey REFERENCES code_events (id) ON DELETE CASCADE,
  ticket_id     uuid CONSTRAINT code_event_links_ticket_id_fkey REFERENCES tickets (id) ON DELETE CASCADE,
  ticket_key    text NOT NULL CONSTRAINT code_event_links_ticket_key_length CHECK (char_length(ticket_key) BETWEEN 1 AND 30),
  outcome       text NOT NULL CONSTRAINT code_event_links_outcome CHECK (outcome IN (
                  'linked', 'in_review', 'fix_committed', 'merged', 'ci_passed', 'deployed', 'reopened',
                  'ticket_closed', 'unknown_ticket', 'other_repository')),
  items         text[] NOT NULL DEFAULT '{}',
  status_from   text,
  status_to     text,
  at            timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT code_event_links_unknown_ticket CHECK ((ticket_id IS NULL) = (outcome = 'unknown_ticket')),
  CONSTRAINT code_event_links_status_pair CHECK ((status_from IS NULL) = (status_to IS NULL))
);
CREATE INDEX code_event_links_ticket_idx ON code_event_links (ticket_id, at) WHERE ticket_id IS NOT NULL;
CREATE INDEX code_event_links_event_idx ON code_event_links (code_event_id);
CREATE INDEX code_event_links_at_idx ON code_event_links (at);
-- An ignored key is logged once per event, however often the event is delivered.
CREATE UNIQUE INDEX code_event_links_ignored_key ON code_event_links (code_event_id, ticket_key)
  WHERE outcome IN ('ticket_closed', 'unknown_ticket', 'other_repository');

-- Which code event met a Definition of Done item, so the checklist can say how.
ALTER TABLE ticket_dod_items ADD COLUMN code_event_id uuid
  CONSTRAINT ticket_dod_items_code_event_id_fkey REFERENCES code_events (id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE ticket_dod_items DROP COLUMN code_event_id;
DROP TABLE code_event_links;
DROP TABLE code_events;
DROP TABLE webhook_deliveries;
ALTER TABLE projects DROP COLUMN repository_id;
DROP TABLE repositories;
