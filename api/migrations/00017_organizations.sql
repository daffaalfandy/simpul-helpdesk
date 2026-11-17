-- +goose Up

-- Owner, 17 Nov: one install can hold several vendor organizations. Each owns its clients,
-- projects, tickets, catalogue, priorities, calendar and repositories. An existing install keeps
-- its data as its first organization; a fresh install has none until first-run setup makes one
-- (its default hours and priorities go: creating an organization inserts its own).
CREATE TABLE organizations (
  id                uuid PRIMARY KEY DEFAULT uuidv7(),
  name              text NOT NULL CONSTRAINT organizations_name_check CHECK (length(name) BETWEEN 1 AND 200),
  ticket_prefix     text NOT NULL CONSTRAINT organizations_ticket_prefix_check CHECK (ticket_prefix ~ '^[A-Z][A-Z0-9]{0,9}$'),
  timezone          text NOT NULL DEFAULT 'Asia/Jakarta',
  footer_note       text,
  logo_key          text CONSTRAINT organizations_logo_key CHECK (logo_key ~ '^[0-9a-f]{32}$'),
  logo_content_type text CONSTRAINT organizations_logo_content_type CHECK (logo_content_type IN ('image/png', 'image/jpeg', 'image/webp')),
  logo_updated_at   timestamptz,
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  -- Ticket keys are unique across the install because prefixes are.
  CONSTRAINT organizations_ticket_prefix_key UNIQUE (ticket_prefix),
  CONSTRAINT organizations_logo_complete CHECK (
    (logo_key IS NULL) = (logo_content_type IS NULL) AND (logo_key IS NULL) = (logo_updated_at IS NULL))
);
CREATE TRIGGER organizations_updated_at BEFORE UPDATE ON organizations
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

INSERT INTO organizations (name, ticket_prefix, timezone, footer_note, logo_key, logo_content_type, logo_updated_at, created_at)
SELECT name, ticket_prefix, timezone, footer_note, logo_key, logo_content_type, logo_updated_at, created_at
FROM organization
WHERE setup_completed_at IS NOT NULL OR EXISTS (SELECT 1 FROM users) OR EXISTS (SELECT 1 FROM clients);

DELETE FROM business_hours WHERE NOT EXISTS (SELECT 1 FROM organizations);
DELETE FROM priorities WHERE NOT EXISTS (SELECT 1 FROM organizations);

-- The install itself keeps only whether first-run setup is done.
ALTER TABLE organization RENAME TO install;
ALTER TABLE install RENAME CONSTRAINT organization_pkey TO install_pkey;
ALTER TABLE install RENAME CONSTRAINT organization_id_check TO install_id_check;
ALTER TRIGGER organization_updated_at ON install RENAME TO install_updated_at;
ALTER TABLE install
  DROP CONSTRAINT organization_logo_complete,
  DROP COLUMN name,
  DROP COLUMN ticket_prefix,
  DROP COLUMN timezone,
  DROP COLUMN footer_note,
  DROP COLUMN logo_key,
  DROP COLUMN logo_content_type,
  DROP COLUMN logo_updated_at;

-- Every organization-owned row names its organization. Existing rows belong to the first one;
-- the updated_at triggers stay off while they are filled in, as nothing about them changed.
ALTER TABLE clients ADD COLUMN organization_id uuid;
ALTER TABLE projects ADD COLUMN organization_id uuid;
ALTER TABLE tickets ADD COLUMN organization_id uuid;
ALTER TABLE catalog_items ADD COLUMN organization_id uuid;
ALTER TABLE priorities ADD COLUMN organization_id uuid;
ALTER TABLE business_hours ADD COLUMN organization_id uuid;
ALTER TABLE holidays ADD COLUMN organization_id uuid;
ALTER TABLE repositories ADD COLUMN organization_id uuid;
ALTER TABLE client_agents ADD COLUMN organization_id uuid;

ALTER TABLE clients DISABLE TRIGGER clients_updated_at;
ALTER TABLE projects DISABLE TRIGGER projects_updated_at;
ALTER TABLE tickets DISABLE TRIGGER tickets_updated_at;
ALTER TABLE catalog_items DISABLE TRIGGER catalog_items_updated_at;
ALTER TABLE priorities DISABLE TRIGGER priorities_updated_at;
ALTER TABLE holidays DISABLE TRIGGER holidays_updated_at;
ALTER TABLE repositories DISABLE TRIGGER repositories_updated_at;
UPDATE clients SET organization_id = (SELECT id FROM organizations);
UPDATE projects SET organization_id = (SELECT id FROM organizations);
UPDATE tickets SET organization_id = (SELECT id FROM organizations);
UPDATE catalog_items SET organization_id = (SELECT id FROM organizations);
UPDATE priorities SET organization_id = (SELECT id FROM organizations);
UPDATE business_hours SET organization_id = (SELECT id FROM organizations);
UPDATE holidays SET organization_id = (SELECT id FROM organizations);
UPDATE repositories SET organization_id = (SELECT id FROM organizations);
UPDATE client_agents SET organization_id = (SELECT id FROM organizations);
ALTER TABLE clients ENABLE TRIGGER clients_updated_at;
ALTER TABLE projects ENABLE TRIGGER projects_updated_at;
ALTER TABLE tickets ENABLE TRIGGER tickets_updated_at;
ALTER TABLE catalog_items ENABLE TRIGGER catalog_items_updated_at;
ALTER TABLE priorities ENABLE TRIGGER priorities_updated_at;
ALTER TABLE holidays ENABLE TRIGGER holidays_updated_at;
ALTER TABLE repositories ENABLE TRIGGER repositories_updated_at;

-- Clients: names unique within an organization.
ALTER TABLE clients
  ALTER COLUMN organization_id SET NOT NULL,
  ADD CONSTRAINT clients_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations (id),
  DROP CONSTRAINT clients_name_key,
  ADD CONSTRAINT clients_name_key UNIQUE (organization_id, name),
  ADD CONSTRAINT clients_id_organization_key UNIQUE (id, organization_id);
CREATE INDEX clients_organization_id_idx ON clients (organization_id);

-- Repositories: the same repository may serve two organizations, each with its own webhook.
ALTER TABLE repositories
  ALTER COLUMN organization_id SET NOT NULL,
  ADD CONSTRAINT repositories_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations (id),
  ADD CONSTRAINT repositories_id_organization_key UNIQUE (id, organization_id);
DROP INDEX repositories_name_key;
CREATE UNIQUE INDEX repositories_name_key ON repositories (organization_id, provider, lower(host), lower(full_name));

-- Catalogue: names unique within an organization.
ALTER TABLE catalog_items
  ALTER COLUMN organization_id SET NOT NULL,
  ADD CONSTRAINT catalog_items_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations (id),
  ADD CONSTRAINT catalog_items_id_organization_key UNIQUE (id, organization_id);
DROP INDEX catalog_items_name_key;
CREATE UNIQUE INDEX catalog_items_name_key ON catalog_items (organization_id, lower(name));

-- Projects belong to their client's organization and read only its repositories.
ALTER TABLE projects
  ALTER COLUMN organization_id SET NOT NULL,
  DROP CONSTRAINT projects_client_id_fkey,
  ADD CONSTRAINT projects_client_id_fkey FOREIGN KEY (client_id, organization_id)
    REFERENCES clients (id, organization_id) ON DELETE CASCADE,
  DROP CONSTRAINT projects_repository_id_fkey,
  ADD CONSTRAINT projects_repository_id_fkey FOREIGN KEY (repository_id, organization_id)
    REFERENCES repositories (id, organization_id) ON DELETE SET NULL (repository_id);

-- Tickets: their client and catalogue item are in the same organization.
ALTER TABLE tickets
  ALTER COLUMN organization_id SET NOT NULL,
  DROP CONSTRAINT tickets_client_id_fkey,
  ADD CONSTRAINT tickets_client_id_fkey FOREIGN KEY (client_id, organization_id)
    REFERENCES clients (id, organization_id) ON DELETE RESTRICT,
  DROP CONSTRAINT tickets_catalog_item_id_fkey,
  ADD CONSTRAINT tickets_catalog_item_id_fkey FOREIGN KEY (catalog_item_id, organization_id)
    REFERENCES catalog_items (id, organization_id) ON DELETE RESTRICT;
DROP INDEX tickets_board_idx;
CREATE INDEX tickets_board_idx ON tickets (organization_id, status, priority, status_changed_at);

-- Priorities: P1–P4 per organization, each with its own names.
ALTER TABLE project_sla_targets
  DROP CONSTRAINT project_sla_targets_priority_fkey,
  ADD CONSTRAINT project_sla_targets_priority_check CHECK (priority IN ('P1', 'P2', 'P3', 'P4'));
ALTER TABLE priorities
  ALTER COLUMN organization_id SET NOT NULL,
  ADD CONSTRAINT priorities_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations (id),
  DROP CONSTRAINT priorities_pkey,
  ADD CONSTRAINT priorities_pkey PRIMARY KEY (organization_id, code);

-- Calendar: each organization keeps its own week and holidays (in its time zone).
ALTER TABLE business_hours
  ALTER COLUMN organization_id SET NOT NULL,
  ADD CONSTRAINT business_hours_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations (id),
  DROP CONSTRAINT business_hours_pkey,
  ADD CONSTRAINT business_hours_pkey PRIMARY KEY (organization_id, weekday);
ALTER TABLE holidays
  ALTER COLUMN organization_id SET NOT NULL,
  ADD CONSTRAINT holidays_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations (id),
  DROP CONSTRAINT holidays_date_key,
  ADD CONSTRAINT holidays_date_key UNIQUE (organization_id, date);

-- Agents link to clients of one organization (their membership there comes next).
ALTER TABLE client_agents
  ALTER COLUMN organization_id SET NOT NULL,
  DROP CONSTRAINT client_agents_client_id_fkey,
  ADD CONSTRAINT client_agents_client_id_fkey FOREIGN KEY (client_id, organization_id)
    REFERENCES clients (id, organization_id) ON DELETE CASCADE;

-- The audit log is append-only: entries written before stay without an organization.
ALTER TABLE audit_log ADD COLUMN organization_id uuid;
CREATE INDEX audit_log_organization_idx ON audit_log (organization_id, at) WHERE organization_id IS NOT NULL;

-- +goose Down
-- Only an install with at most one organization can go back.
-- +goose StatementBegin
DO $$
BEGIN
  IF (SELECT count(*) FROM organizations) > 1 THEN
    RAISE EXCEPTION 'cannot go back to one organization: % organizations exist', (SELECT count(*) FROM organizations);
  END IF;
END
$$;
-- +goose StatementEnd
DROP INDEX audit_log_organization_idx;
ALTER TABLE audit_log DROP COLUMN organization_id;
ALTER TABLE client_agents
  DROP CONSTRAINT client_agents_client_id_fkey,
  ADD CONSTRAINT client_agents_client_id_fkey FOREIGN KEY (client_id) REFERENCES clients (id) ON DELETE CASCADE,
  DROP COLUMN organization_id;
ALTER TABLE holidays DROP CONSTRAINT holidays_date_key, ADD CONSTRAINT holidays_date_key UNIQUE (date), DROP COLUMN organization_id;
ALTER TABLE business_hours DROP CONSTRAINT business_hours_pkey, ADD CONSTRAINT business_hours_pkey PRIMARY KEY (weekday), DROP COLUMN organization_id;
ALTER TABLE priorities DROP CONSTRAINT priorities_pkey, ADD CONSTRAINT priorities_pkey PRIMARY KEY (code), DROP COLUMN organization_id;
ALTER TABLE project_sla_targets
  DROP CONSTRAINT project_sla_targets_priority_check,
  ADD CONSTRAINT project_sla_targets_priority_fkey FOREIGN KEY (priority) REFERENCES priorities (code);
DROP INDEX tickets_board_idx;
CREATE INDEX tickets_board_idx ON tickets (status, priority, status_changed_at);
ALTER TABLE tickets
  DROP CONSTRAINT tickets_catalog_item_id_fkey,
  ADD CONSTRAINT tickets_catalog_item_id_fkey FOREIGN KEY (catalog_item_id) REFERENCES catalog_items (id) ON DELETE RESTRICT,
  DROP CONSTRAINT tickets_client_id_fkey,
  ADD CONSTRAINT tickets_client_id_fkey FOREIGN KEY (client_id) REFERENCES clients (id) ON DELETE RESTRICT,
  DROP COLUMN organization_id;
ALTER TABLE projects
  DROP CONSTRAINT projects_repository_id_fkey,
  ADD CONSTRAINT projects_repository_id_fkey FOREIGN KEY (repository_id) REFERENCES repositories (id) ON DELETE SET NULL,
  DROP CONSTRAINT projects_client_id_fkey,
  ADD CONSTRAINT projects_client_id_fkey FOREIGN KEY (client_id) REFERENCES clients (id) ON DELETE CASCADE,
  DROP COLUMN organization_id;
DROP INDEX catalog_items_name_key;
CREATE UNIQUE INDEX catalog_items_name_key ON catalog_items (lower(name));
ALTER TABLE catalog_items DROP CONSTRAINT catalog_items_id_organization_key, DROP COLUMN organization_id;
DROP INDEX repositories_name_key;
CREATE UNIQUE INDEX repositories_name_key ON repositories (provider, lower(host), lower(full_name));
ALTER TABLE repositories DROP CONSTRAINT repositories_id_organization_key, DROP COLUMN organization_id;
DROP INDEX clients_organization_id_idx;
ALTER TABLE clients
  DROP CONSTRAINT clients_id_organization_key,
  DROP CONSTRAINT clients_name_key,
  ADD CONSTRAINT clients_name_key UNIQUE (name),
  DROP COLUMN organization_id;
ALTER TABLE install
  ADD COLUMN name text NOT NULL DEFAULT 'SimDesk' CONSTRAINT organization_name_check CHECK (length(name) BETWEEN 1 AND 200),
  ADD COLUMN ticket_prefix text NOT NULL DEFAULT 'HD' CONSTRAINT organization_ticket_prefix_check CHECK (ticket_prefix ~ '^[A-Z][A-Z0-9]{0,9}$'),
  ADD COLUMN timezone text NOT NULL DEFAULT 'Asia/Jakarta',
  ADD COLUMN footer_note text,
  ADD COLUMN logo_key text CONSTRAINT organization_logo_key CHECK (logo_key ~ '^[0-9a-f]{32}$'),
  ADD COLUMN logo_content_type text
    CONSTRAINT organization_logo_content_type CHECK (logo_content_type IN ('image/png', 'image/jpeg', 'image/webp')),
  ADD COLUMN logo_updated_at timestamptz,
  ADD CONSTRAINT organization_logo_complete CHECK (
    (logo_key IS NULL) = (logo_content_type IS NULL) AND (logo_key IS NULL) = (logo_updated_at IS NULL));
UPDATE install i SET name = o.name, ticket_prefix = o.ticket_prefix, timezone = o.timezone, footer_note = o.footer_note,
  logo_key = o.logo_key, logo_content_type = o.logo_content_type, logo_updated_at = o.logo_updated_at
FROM organizations o;
ALTER TRIGGER install_updated_at ON install RENAME TO organization_updated_at;
ALTER TABLE install RENAME CONSTRAINT install_id_check TO organization_id_check;
ALTER TABLE install RENAME CONSTRAINT install_pkey TO organization_pkey;
ALTER TABLE install RENAME TO organization;
INSERT INTO priorities (code, name, description)
SELECT * FROM (VALUES
  ('P1', 'Critical', 'A public service or the whole office is down'),
  ('P2', 'High', 'A team or department cannot work'),
  ('P3', 'Standard', 'One person is slowed down, with a workaround'),
  ('P4', 'Low', 'Minor issue or question, no rush')) AS v (code, name, description)
WHERE NOT EXISTS (SELECT 1 FROM priorities);
INSERT INTO business_hours (weekday, open_minute, close_minute)
SELECT d, 480, 1020 FROM generate_series(1, 5) AS d WHERE NOT EXISTS (SELECT 1 FROM business_hours);
DROP TABLE organizations;
