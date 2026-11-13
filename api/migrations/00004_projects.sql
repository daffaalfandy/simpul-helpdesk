-- +goose Up

-- Priority names and descriptions requesters see, shared by every project (follow-up #1).
-- The four rows are fixed; admins only rename them.
CREATE TABLE priorities (
  code        text PRIMARY KEY CHECK (code IN ('P1', 'P2', 'P3', 'P4')),
  name        text NOT NULL CHECK (length(name) BETWEEN 1 AND 40),
  description text NOT NULL CHECK (length(description) BETWEEN 1 AND 200),
  updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER priorities_updated_at BEFORE UPDATE ON priorities
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

INSERT INTO priorities (code, name, description) VALUES
  ('P1', 'Critical', 'A public service or the whole office is down'),
  ('P2', 'High', 'A team or department cannot work'),
  ('P3', 'Standard', 'One person is slowed down, with a workaround'),
  ('P4', 'Low', 'Minor issue or question, no rush');

-- A project ties a client to its SLA (and later a repository and Definition of Done). The
-- portal's "Which system?" lists a client's active projects; service requests go to the
-- client's default project.
CREATE TABLE projects (
  id          uuid PRIMARY KEY DEFAULT uuidv7(),
  client_id   uuid NOT NULL CONSTRAINT projects_client_id_fkey REFERENCES clients (id) ON DELETE CASCADE,
  name        text NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
  description text CHECK (length(description) BETWEEN 1 AND 300),
  is_default  boolean NOT NULL DEFAULT false,
  active      boolean NOT NULL DEFAULT true,
  sla_pause_on_waiting_client     boolean NOT NULL DEFAULT true,
  sla_pause_on_uat                boolean NOT NULL DEFAULT true,
  sla_at_risk_percent             smallint NOT NULL DEFAULT 75 CHECK (sla_at_risk_percent IN (50, 75, 90)),
  sla_notify_supervisor_on_breach boolean NOT NULL DEFAULT true,
  sla_notify_pic_on_p1_breach     boolean NOT NULL DEFAULT false,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  -- Lets tickets reference (project, client) together, so a ticket's project always belongs to
  -- the ticket's client and a project with tickets can't move to another client.
  CONSTRAINT projects_id_client_key UNIQUE (id, client_id),
  -- The default receives service requests, so it can't be hidden from the report form.
  CONSTRAINT projects_default_active CHECK (NOT is_default OR active)
);
CREATE UNIQUE INDEX projects_name_key ON projects (client_id, lower(name));
CREATE UNIQUE INDEX projects_single_default_key ON projects (client_id) WHERE is_default;
CREATE TRIGGER projects_updated_at BEFORE UPDATE ON projects
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- One row per priority. Targets are stored as written ("2 working days"); the pure sla package
-- converts them. 525600 minutes = one year, the longest target in any unit.
CREATE TABLE project_sla_targets (
  project_id uuid NOT NULL CONSTRAINT project_sla_targets_project_id_fkey REFERENCES projects (id) ON DELETE CASCADE,
  priority   text NOT NULL CONSTRAINT project_sla_targets_priority_fkey REFERENCES priorities (code),
  first_response_value integer NOT NULL CHECK (first_response_value BETWEEN 1 AND 525600),
  first_response_unit  text NOT NULL CHECK (first_response_unit IN ('min', 'h', 'wd', 'd')),
  resolve_value        integer NOT NULL CHECK (resolve_value BETWEEN 1 AND 525600),
  resolve_unit         text NOT NULL CHECK (resolve_unit IN ('min', 'h', 'wd', 'd')),
  deliver_value        integer NOT NULL CHECK (deliver_value BETWEEN 1 AND 525600),
  deliver_unit         text NOT NULL CHECK (deliver_unit IN ('min', 'h', 'wd', 'd')),
  calendar   text NOT NULL CHECK (calendar IN ('business_hours', 'always')),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (project_id, priority)
);
CREATE TRIGGER project_sla_targets_updated_at BEFORE UPDATE ON project_sla_targets
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Deferred rules come last: once a deferred trigger has fired, Postgres refuses further DDL on
-- that table in the same transaction.

-- +goose StatementBegin
CREATE FUNCTION project_targets_complete() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
  project uuid;
BEGIN
  IF TG_TABLE_NAME = 'projects' THEN
    project := NEW.id;
  ELSE
    project := OLD.project_id;
  END IF;
  -- 4 = one target per priority, P1 to P4, for as long as the project exists.
  IF EXISTS (SELECT 1 FROM projects WHERE id = project)
     AND (SELECT count(*) FROM project_sla_targets WHERE project_id = project) <> 4 THEN
    RAISE EXCEPTION 'a project needs an SLA target for every priority'
      USING ERRCODE = 'check_violation', CONSTRAINT = 'project_targets_complete';
  END IF;
  RETURN NULL;
END
$$;
-- +goose StatementEnd
CREATE CONSTRAINT TRIGGER projects_targets_complete AFTER INSERT ON projects
  DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION project_targets_complete();
CREATE CONSTRAINT TRIGGER project_sla_targets_complete AFTER DELETE OR UPDATE OF project_id, priority ON project_sla_targets
  DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION project_targets_complete();

-- +goose StatementBegin
CREATE FUNCTION projects_default_exists() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
  client uuid;
BEGIN
  IF TG_OP = 'DELETE' THEN
    client := OLD.client_id;
  ELSE
    client := NEW.client_id;
  END IF;
  IF EXISTS (SELECT 1 FROM projects WHERE client_id = client AND active)
     AND NOT EXISTS (SELECT 1 FROM projects WHERE client_id = client AND is_default) THEN
    RAISE EXCEPTION 'a client with active projects needs a default project'
      USING ERRCODE = 'check_violation', CONSTRAINT = 'projects_default_exists';
  END IF;
  RETURN NULL;
END
$$;
-- +goose StatementEnd
CREATE CONSTRAINT TRIGGER projects_default_exists AFTER INSERT OR DELETE OR UPDATE OF is_default, active ON projects
  DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION projects_default_exists();

-- +goose Down
DROP TABLE project_sla_targets;
DROP TABLE projects;
DROP TABLE priorities;
DROP FUNCTION projects_default_exists();
DROP FUNCTION project_targets_complete();
