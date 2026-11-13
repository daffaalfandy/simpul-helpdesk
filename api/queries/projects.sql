-- name: ListProjects :many
-- client_id NULL lists every client's projects; agent_id NULL skips the agent scope.
SELECT p.id, p.client_id, c.name AS client_name, p.name, p.description, p.is_default, p.active,
       p.sla_pause_on_waiting_client, p.sla_pause_on_uat, p.sla_at_risk_percent,
       p.sla_notify_supervisor_on_breach, p.sla_notify_pic_on_p1_breach, p.created_at, p.updated_at,
       (SELECT count(*) FROM tickets t
        WHERE t.project_id = p.id AND t.status NOT IN ('resolved', 'closed'))::int AS open_ticket_count
FROM projects p
JOIN clients c ON c.id = p.client_id
WHERE (sqlc.narg(client_id)::uuid IS NULL OR p.client_id = sqlc.narg(client_id)::uuid)
  AND (sqlc.narg(agent_id)::uuid IS NULL OR EXISTS (
        SELECT 1 FROM client_agents ca WHERE ca.client_id = p.client_id AND ca.user_id = sqlc.narg(agent_id)::uuid))
ORDER BY lower(c.name), lower(p.name);

-- name: GetProject :one
SELECT p.id, p.client_id, c.name AS client_name, p.name, p.description, p.is_default, p.active,
       p.sla_pause_on_waiting_client, p.sla_pause_on_uat, p.sla_at_risk_percent,
       p.sla_notify_supervisor_on_breach, p.sla_notify_pic_on_p1_breach, p.created_at, p.updated_at,
       (SELECT count(*) FROM tickets t
        WHERE t.project_id = p.id AND t.status NOT IN ('resolved', 'closed'))::int AS open_ticket_count
FROM projects p
JOIN clients c ON c.id = p.client_id
WHERE p.id = @id
  AND (sqlc.narg(agent_id)::uuid IS NULL OR EXISTS (
        SELECT 1 FROM client_agents ca WHERE ca.client_id = p.client_id AND ca.user_id = sqlc.narg(agent_id)::uuid));

-- name: GetProjectForUpdate :one
SELECT id, client_id, name, description, is_default, active,
       sla_pause_on_waiting_client, sla_pause_on_uat, sla_at_risk_percent,
       sla_notify_supervisor_on_breach, sla_notify_pic_on_p1_breach
FROM projects WHERE id = @id FOR UPDATE;

-- name: ListProjectSlaTargets :many
-- project_id NULL lists every project's targets.
SELECT project_id, priority, first_response_value, first_response_unit, resolve_value, resolve_unit,
       deliver_value, deliver_unit, calendar
FROM project_sla_targets
WHERE sqlc.narg(project_id)::uuid IS NULL OR project_id = sqlc.narg(project_id)::uuid
ORDER BY project_id, priority;

-- name: CountClientProjects :one
SELECT count(*)::int FROM projects WHERE client_id = @client_id;

-- name: CountOtherActiveProjects :one
SELECT count(*)::int FROM projects WHERE client_id = @client_id AND active AND id <> @id;

-- name: CreateProject :one
INSERT INTO projects (client_id, name, description, is_default, active)
VALUES (@client_id, @name, sqlc.narg(description), @is_default, @active)
RETURNING id;

-- name: UpdateProject :exec
UPDATE projects SET name = @name, description = sqlc.narg(description), active = @active WHERE id = @id;

-- name: ClearClientDefaultProject :exec
-- The single-default index can't be deferred, so a switch clears the old default first.
UPDATE projects SET is_default = false WHERE client_id = @client_id AND is_default AND id <> @keep_id;

-- name: MarkProjectDefault :exec
UPDATE projects SET is_default = true WHERE id = @id;

-- name: UpdateProjectSlaSettings :exec
UPDATE projects
SET sla_pause_on_waiting_client = @pause_on_waiting_client,
    sla_pause_on_uat = @pause_on_uat,
    sla_at_risk_percent = @at_risk_percent,
    sla_notify_supervisor_on_breach = @notify_supervisor_on_breach,
    sla_notify_pic_on_p1_breach = @notify_pic_on_p1_breach
WHERE id = @id;

-- name: UpsertProjectSlaTarget :exec
INSERT INTO project_sla_targets (project_id, priority, first_response_value, first_response_unit,
                                 resolve_value, resolve_unit, deliver_value, deliver_unit, calendar)
VALUES (@project_id, @priority, @first_response_value, @first_response_unit,
        @resolve_value, @resolve_unit, @deliver_value, @deliver_unit, @calendar)
ON CONFLICT (project_id, priority) DO UPDATE
SET first_response_value = EXCLUDED.first_response_value, first_response_unit = EXCLUDED.first_response_unit,
    resolve_value = EXCLUDED.resolve_value, resolve_unit = EXCLUDED.resolve_unit,
    deliver_value = EXCLUDED.deliver_value, deliver_unit = EXCLUDED.deliver_unit,
    calendar = EXCLUDED.calendar;

-- name: DeleteProject :exec
DELETE FROM projects WHERE id = @id;

-- name: GetProjectByName :one
-- Seed only.
SELECT id FROM projects WHERE client_id = @client_id AND lower(name) = lower(@name);

-- name: CountProjectTickets :one
SELECT count(*)::int FROM tickets WHERE project_id = @project_id;

-- name: GetActiveClientProject :one
-- A project a new incident may name: active and belonging to the ticket's client.
SELECT id, name FROM projects WHERE id = @id AND client_id = @client_id AND active;

-- name: GetClientDefaultProject :one
SELECT id, name FROM projects WHERE client_id = @client_id AND is_default;

-- name: ListActiveClientProjects :many
-- The report form's "Which system?" choices.
SELECT id, name, description FROM projects WHERE client_id = @client_id AND active ORDER BY lower(name);
