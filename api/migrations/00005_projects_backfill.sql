-- +goose Up

-- Follow-up #1, contract step: tickets move from client SLA plans and the organisation-wide
-- systems list to projects. Existing data (development databases only, before v1) carries over:
--   * every client with tickets or requesters and no default project gets "General support",
--     its default, with its SLA plan's targets;
--   * every ticket points at its client's default project, and its SLA snapshot is rewritten in
--     the project format (version 2) from the targets it was frozen with;
--   * ticket systems are dropped (their ids stay in the audit log), and so is the catalogue's
--     delivery time: a project's deliver target replaces it.
-- The backfill writes no audit entries and keeps every ticket's updated_at.

-- Run the deferred project rules after each statement, so the DDL below doesn't meet pending
-- trigger events on the tables the backfill writes.
SET CONSTRAINTS ALL IMMEDIATE;

-- Converts stored minutes to how people write targets: whole working days on business hours,
-- whole days on 24/7, else whole hours, else minutes. Dropped at the end of this migration.
-- +goose StatementBegin
CREATE FUNCTION m3b_duration(minutes integer, calendar text, working_day integer) RETURNS jsonb
LANGUAGE sql IMMUTABLE AS $$
  SELECT CASE
    WHEN calendar = 'business_hours' AND minutes % working_day = 0
      THEN jsonb_build_object('value', minutes / working_day, 'unit', 'wd')
    -- 1440 minutes = one calendar day.
    WHEN calendar = 'always' AND minutes % 1440 = 0
      THEN jsonb_build_object('value', minutes / 1440, 'unit', 'd')
    WHEN minutes % 60 = 0
      THEN jsonb_build_object('value', minutes / 60, 'unit', 'h')
    ELSE jsonb_build_object('value', minutes, 'unit', 'min')
  END
$$;
-- +goose StatementEnd

-- The working day is the business-hours window; 540 minutes (08:00–17:00) if none is stored.
CREATE TEMP TABLE m3b_working_day ON COMMIT DROP AS
  SELECT COALESCE(max(close_minute - open_minute), 540)::integer AS minutes FROM business_hours;

WITH new_projects AS (
  INSERT INTO projects (client_id, name, is_default)
  SELECT c.id, 'General support', true
  FROM clients c
  WHERE (EXISTS (SELECT 1 FROM tickets t WHERE t.client_id = c.id)
         OR EXISTS (SELECT 1 FROM users u WHERE u.client_id = c.id))
    AND NOT EXISTS (SELECT 1 FROM projects p WHERE p.client_id = c.id AND p.is_default)
  RETURNING id, client_id
)
INSERT INTO project_sla_targets (project_id, priority, first_response_value, first_response_unit,
                                 resolve_value, resolve_unit, deliver_value, deliver_unit, calendar)
SELECT np.id, x.priority,
       (x.fr ->> 'value')::integer, x.fr ->> 'unit',
       (x.rs ->> 'value')::integer, x.rs ->> 'unit',
       (x.dl ->> 'value')::integer, x.dl ->> 'unit',
       x.calendar
FROM new_projects np
JOIN clients c ON c.id = np.client_id
CROSS JOIN m3b_working_day wd
CROSS JOIN LATERAL (
  SELECT t.priority, t.calendar,
         m3b_duration(t.first_response_minutes, t.calendar, wd.minutes) AS fr,
         m3b_duration(t.resolve_minutes, t.calendar, wd.minutes) AS rs,
         m3b_duration(t.deliver_minutes, t.calendar, wd.minutes) AS dl
  FROM sla_targets t
  WHERE t.plan_id = c.sla_plan_id
) x;

ALTER TABLE tickets ADD COLUMN project_id uuid;

ALTER TABLE tickets DISABLE TRIGGER tickets_updated_at;
UPDATE tickets t
SET project_id = p.id,
    sla_snapshot = jsonb_build_object(
      'version', 2,
      'projectId', p.id,
      'projectName', p.name,
      'targets', COALESCE((
        SELECT jsonb_agg(jsonb_build_object(
                 'priority', x ->> 'priority',
                 'firstResponse', m3b_duration((x ->> 'firstResponseMinutes')::integer, x ->> 'calendar', wd.minutes),
                 'resolve', m3b_duration((x ->> 'resolveMinutes')::integer, x ->> 'calendar', wd.minutes),
                 'deliver', m3b_duration(
                   CASE WHEN (x ->> 'deliverFromCatalog')::boolean AND t.sla_snapshot ->> 'catalogDeliverMinutes' IS NOT NULL
                        THEN (t.sla_snapshot ->> 'catalogDeliverMinutes')::integer
                        ELSE (x ->> 'deliverMinutes')::integer END,
                   x ->> 'calendar', wd.minutes),
                 'calendar', x ->> 'calendar')
               ORDER BY x ->> 'priority')
        FROM jsonb_array_elements(t.sla_snapshot -> 'targets') x
      ), '[]'::jsonb),
      'pauseOnWaitingClient', p.sla_pause_on_waiting_client,
      'pauseOnUat', p.sla_pause_on_uat,
      'atRiskPercent', p.sla_at_risk_percent,
      'notifySupervisorOnBreach', p.sla_notify_supervisor_on_breach,
      'notifyPicOnP1Breach', p.sla_notify_pic_on_p1_breach)
FROM projects p, m3b_working_day wd
WHERE p.client_id = t.client_id AND p.is_default;
ALTER TABLE tickets ENABLE TRIGGER tickets_updated_at;

ALTER TABLE tickets ALTER COLUMN project_id SET NOT NULL;
-- With (project, client) together a ticket's project always belongs to its client.
ALTER TABLE tickets ADD CONSTRAINT tickets_project_fkey
  FOREIGN KEY (project_id, client_id) REFERENCES projects (id, client_id) ON DELETE RESTRICT;
CREATE INDEX tickets_project_id_idx ON tickets (project_id);

-- Dropping system_id also drops tickets_type_detail, which named it; only requests name a
-- catalogue item, so that half comes back.
ALTER TABLE tickets DROP COLUMN system_id;
ALTER TABLE tickets ADD CONSTRAINT tickets_type_detail CHECK (type = 'request' OR catalog_item_id IS NULL);
DROP TABLE systems;

ALTER TABLE clients DROP COLUMN sla_plan_id;
DROP FUNCTION default_sla_plan_id();
DROP TABLE sla_targets;
DROP TABLE sla_plans;
DROP FUNCTION sla_plan_targets_complete();
DROP FUNCTION sla_plans_default_exists();

ALTER TABLE catalog_items DROP COLUMN delivery_minutes;

DROP FUNCTION m3b_duration(integer, text, integer);

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
  RAISE EXCEPTION 'migration 00005 drops SLA plans and systems and can''t be undone; reset the database instead';
END
$$;
-- +goose StatementEnd
