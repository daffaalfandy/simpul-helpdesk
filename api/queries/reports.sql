-- name: ListReportTickets :many
-- Tickets created in [month_start, month_end), with their SLA outcome so far. A ticket the SLA
-- scan hasn't replayed yet has no ticket_sla row: no breach and no first-response time.
-- project_id narrows the list to one project; NULL lists all.
SELECT t.id, t.key, t.client_id, c.name AS client_name, t.project_id, p.name AS project_name, t.type, t.channel, t.priority,
       t.status, t.subject, t.created_at, t.first_response_at, t.resolved_at,
       s.first_response_elapsed_ms, s.first_response_met_at,
       s.first_response_breached_at, s.resolution_breached_at,
       -- ticket.SLASnapshot embeds sla.ProjectSLA, whose JSON tag is pauseOnWaitingClient.
       coalesce((t.sla_snapshot->>'pauseOnWaitingClient')::boolean, false)::boolean AS pauses_while_waiting
FROM tickets t
JOIN projects p ON p.id = t.project_id
JOIN clients c ON c.id = t.client_id
LEFT JOIN ticket_sla s ON s.ticket_id = t.id
WHERE t.organization_id = @organization_id
  AND t.created_at >= @month_start::timestamptz
  AND t.created_at < @month_end::timestamptz
  AND (sqlc.narg(project_id)::uuid IS NULL OR t.project_id = sqlc.narg(project_id)::uuid)
ORDER BY t.created_at, t.number;

-- name: ListReportProjects :many
-- Every project with its client and the client's contract, hidden ones included.
SELECT p.id, p.name, p.active, p.client_id, c.name AS client_name, c.contract_no, c.contract_start, c.contract_end
FROM projects p
JOIN clients c ON c.id = p.client_id
WHERE p.organization_id = @organization_id
ORDER BY lower(c.name), lower(p.name);

-- name: GetReportProject :one
SELECT p.id, p.name, p.active, p.client_id, c.name AS client_name, c.contract_no, c.contract_start, c.contract_end
FROM projects p
JOIN clients c ON c.id = p.client_id
WHERE p.id = @id AND p.organization_id = @organization_id;

-- name: GetFirstTicketCreatedAt :one
-- No row for an organization without tickets.
SELECT created_at FROM tickets WHERE organization_id = @organization_id ORDER BY created_at LIMIT 1;
