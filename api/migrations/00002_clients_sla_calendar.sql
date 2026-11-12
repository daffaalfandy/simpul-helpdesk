-- +goose Up

-- SLA plans: named sets of per-priority targets. Clients pick one; exactly one is the default.
CREATE TABLE sla_plans (
  id         uuid PRIMARY KEY DEFAULT uuidv7(),
  name       text NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
  is_default boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX sla_plans_name_key ON sla_plans (lower(name));
-- At most one default; the deferred trigger below makes it exactly one.
CREATE UNIQUE INDEX sla_plans_single_default_key ON sla_plans (is_default) WHERE is_default;
CREATE TRIGGER sla_plans_updated_at BEFORE UPDATE ON sla_plans
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Minutes are working minutes when calendar = 'business_hours', wall minutes when 'always'.
CREATE TABLE sla_targets (
  plan_id                uuid NOT NULL REFERENCES sla_plans (id) ON DELETE CASCADE,
  priority               text NOT NULL CHECK (priority IN ('P1', 'P2', 'P3', 'P4')),
  first_response_minutes integer NOT NULL CHECK (first_response_minutes > 0),
  resolve_minutes        integer NOT NULL CHECK (resolve_minutes > 0),
  deliver_minutes        integer NOT NULL CHECK (deliver_minutes > 0),
  -- Service requests take the catalogue item's delivery time when set; deliver_minutes is the fallback.
  deliver_from_catalog   boolean NOT NULL DEFAULT false,
  calendar               text NOT NULL CHECK (calendar IN ('business_hours', 'always')),
  updated_at             timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (plan_id, priority),
  CONSTRAINT sla_targets_response_first CHECK (
    first_response_minutes <= resolve_minutes AND first_response_minutes <= deliver_minutes)
);
CREATE TRIGGER sla_targets_updated_at BEFORE UPDATE ON sla_targets
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose StatementBegin
CREATE FUNCTION sla_plans_default_exists() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM sla_plans WHERE is_default) THEN
    RAISE EXCEPTION 'one SLA plan must be the default'
      USING ERRCODE = 'check_violation', CONSTRAINT = 'sla_plans_default_exists';
  END IF;
  RETURN NULL;
END
$$;
-- +goose StatementEnd
CREATE CONSTRAINT TRIGGER sla_plans_default_exists AFTER UPDATE OF is_default OR DELETE ON sla_plans
  DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION sla_plans_default_exists();

-- +goose StatementBegin
CREATE FUNCTION sla_plan_targets_complete() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
  plan uuid;
BEGIN
  IF TG_TABLE_NAME = 'sla_plans' THEN
    plan := NEW.id;
  ELSE
    plan := OLD.plan_id;
  END IF;
  -- 4 = one target per priority, P1 to P4, for as long as the plan exists.
  IF EXISTS (SELECT 1 FROM sla_plans WHERE id = plan)
     AND (SELECT count(*) FROM sla_targets WHERE plan_id = plan) <> 4 THEN
    RAISE EXCEPTION 'an SLA plan needs a target for every priority'
      USING ERRCODE = 'check_violation', CONSTRAINT = 'sla_plan_targets_complete';
  END IF;
  RETURN NULL;
END
$$;
-- +goose StatementEnd
CREATE CONSTRAINT TRIGGER sla_plans_targets_complete AFTER INSERT ON sla_plans
  DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION sla_plan_targets_complete();
CREATE CONSTRAINT TRIGGER sla_targets_complete AFTER DELETE OR UPDATE OF plan_id, priority ON sla_targets
  DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION sla_plan_targets_complete();

-- The built-in default plan (HANDOFF §6). Every install needs one; it is not demo data.
-- 540 minutes = 1 working day of 08:00–17:00.
INSERT INTO sla_plans (name, is_default) VALUES ('Standard', true);
INSERT INTO sla_targets (plan_id, priority, first_response_minutes, resolve_minutes, deliver_minutes,
                         deliver_from_catalog, calendar)
SELECT p.id, t.priority, t.first_response, t.resolve, t.deliver, t.from_catalog, t.calendar
FROM sla_plans p,
     (VALUES ('P1',  15,  240,  240, false, 'always'),
             ('P2',  30,  480,  540, false, 'business_hours'),
             ('P3', 120, 1080, 1080, true,  'business_hours'),
             ('P4', 240, 2700, 2700, false, 'business_hours'))
       AS t (priority, first_response, resolve, deliver, from_catalog, calendar)
WHERE p.is_default;

-- Weekly working hours in organization.timezone. weekday: 0 = Sunday … 6 = Saturday, as Go's
-- time.Weekday and Postgres' dow. A missing weekday is closed. Minutes from midnight; 1440 = 24:00.
CREATE TABLE business_hours (
  weekday      smallint PRIMARY KEY CHECK (weekday BETWEEN 0 AND 6),
  open_minute  smallint NOT NULL CHECK (open_minute >= 0),
  close_minute smallint NOT NULL CHECK (close_minute <= 1440),
  CONSTRAINT business_hours_window CHECK (open_minute < close_minute)
);
-- Mon–Fri 08:00–17:00 (HANDOFF §6).
INSERT INTO business_hours (weekday, open_minute, close_minute)
SELECT d, 480, 1020 FROM generate_series(1, 5) AS d;

-- Days without working time (national holidays, cuti bersama). Civil dates in organization.timezone.
CREATE TABLE holidays (
  id         uuid PRIMARY KEY DEFAULT uuidv7(),
  date       date NOT NULL CONSTRAINT holidays_date_key UNIQUE,
  name       text NOT NULL CHECK (length(name) BETWEEN 1 AND 200),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER holidays_updated_at BEFORE UPDATE ON holidays
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose StatementBegin
CREATE FUNCTION default_sla_plan_id() RETURNS uuid LANGUAGE sql STABLE AS $$
  SELECT id FROM sla_plans WHERE is_default
$$;
-- +goose StatementEnd

ALTER TABLE clients
  ADD COLUMN contract_no    text CHECK (length(contract_no) BETWEEN 1 AND 100),
  ADD COLUMN contract_start date,
  ADD COLUMN contract_end   date,
  ADD COLUMN pic_name       text CHECK (length(pic_name) BETWEEN 1 AND 200),
  ADD COLUMN pic_email      text CHECK (pic_email = lower(pic_email) AND position('@' IN pic_email) > 1),
  -- A contract term: a client keeps its plan when the default changes.
  ADD COLUMN sla_plan_id    uuid NOT NULL DEFAULT default_sla_plan_id()
    CONSTRAINT clients_sla_plan_id_fkey REFERENCES sla_plans (id) ON DELETE RESTRICT,
  ADD CONSTRAINT clients_contract_period CHECK (contract_end >= contract_start);
CREATE INDEX clients_sla_plan_id_idx ON clients (sla_plan_id);

-- Agents work the clients they are assigned to (HANDOFF §3); supervisors and admins see all.
-- Only users with role 'agent' are assigned; the service checks the role.
CREATE TABLE client_agents (
  client_id  uuid NOT NULL REFERENCES clients (id) ON DELETE CASCADE,
  user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (client_id, user_id)
);
CREATE INDEX client_agents_user_id_idx ON client_agents (user_id);

-- +goose Down
DROP TABLE client_agents;
ALTER TABLE clients
  DROP CONSTRAINT clients_contract_period,
  DROP COLUMN sla_plan_id,
  DROP COLUMN pic_email,
  DROP COLUMN pic_name,
  DROP COLUMN contract_end,
  DROP COLUMN contract_start,
  DROP COLUMN contract_no;
DROP FUNCTION default_sla_plan_id();
DROP TABLE holidays;
DROP TABLE business_hours;
DROP TABLE sla_targets;
DROP TABLE sla_plans;
DROP FUNCTION sla_plan_targets_complete();
DROP FUNCTION sla_plans_default_exists();
