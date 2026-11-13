-- Scoped queries take scope_client_id (requesters: their client) and scope_agent_id (agents: their
-- assigned clients); both are NULL for supervisors and admins. Only internal/tickets/access.go sets them.

-- name: NextTicketNumber :one
SELECT nextval('ticket_number_seq')::bigint;

-- name: CreateTicket :one
INSERT INTO tickets (number, key, type, status, priority, channel, client_id, requester_id, assignee_id,
                     created_by, project_id, catalog_item_id, subject, description, sla_snapshot)
VALUES (@number, @key, @type, @status, @priority, @channel, @client_id, @requester_id, sqlc.narg(assignee_id),
        @created_by, @project_id, sqlc.narg(catalog_item_id), @subject, @description, @sla_snapshot)
RETURNING id, created_at;

-- name: ListTickets :many
-- Newest first. before pages by id (uuidv7, so creation order). search_key matches a key
-- exactly; search_pattern is an escaped ILIKE pattern over the subject.
SELECT t.id, t.key, t.type, t.status, t.priority, t.subject, t.created_at, t.updated_at, t.resolved_at,
       t.client_id, c.name AS client_name, t.requester_id, r.name AS requester_name,
       t.assignee_id, a.name AS assignee_name
FROM tickets t
JOIN clients c ON c.id = t.client_id
JOIN users r ON r.id = t.requester_id
LEFT JOIN users a ON a.id = t.assignee_id
WHERE (sqlc.narg(scope_client_id)::uuid IS NULL OR t.client_id = sqlc.narg(scope_client_id)::uuid)
  AND (sqlc.narg(scope_agent_id)::uuid IS NULL
       OR EXISTS (SELECT 1 FROM client_agents ca
                  WHERE ca.client_id = t.client_id AND ca.user_id = sqlc.narg(scope_agent_id)::uuid))
  AND t.status = ANY(@statuses::text[])
  AND (sqlc.narg(client_id)::uuid IS NULL OR t.client_id = sqlc.narg(client_id)::uuid)
  AND (sqlc.narg(assignee_id)::uuid IS NULL OR t.assignee_id = sqlc.narg(assignee_id)::uuid)
  AND (NOT @unassigned::boolean OR t.assignee_id IS NULL)
  AND (sqlc.narg(search_pattern)::text IS NULL
       OR t.key = sqlc.narg(search_key)::text
       OR t.subject ILIKE sqlc.narg(search_pattern)::text)
  AND (sqlc.narg(before)::uuid IS NULL OR t.id < sqlc.narg(before)::uuid)
ORDER BY t.id DESC
LIMIT @row_limit;

-- name: GetTicket :one
SELECT t.id, t.key, t.type, t.status, t.priority, t.channel, t.subject, t.description, t.sla_snapshot,
       t.first_response_at, t.resolved_at, t.closed_at, t.created_at, t.updated_at,
       t.client_id, c.name AS client_name,
       t.requester_id, r.name AS requester_name, r.job_title AS requester_job_title,
       t.assignee_id, a.name AS assignee_name,
       t.created_by, cb.name AS created_by_name, cb.role AS created_by_role,
       t.project_id, p.name AS project_name,
       t.catalog_item_id, ci.name AS catalog_item_name
FROM tickets t
JOIN clients c ON c.id = t.client_id
JOIN users r ON r.id = t.requester_id
JOIN users cb ON cb.id = t.created_by
LEFT JOIN users a ON a.id = t.assignee_id
JOIN projects p ON p.id = t.project_id
LEFT JOIN catalog_items ci ON ci.id = t.catalog_item_id
WHERE t.key = @key
  AND (sqlc.narg(scope_client_id)::uuid IS NULL OR t.client_id = sqlc.narg(scope_client_id)::uuid)
  AND (sqlc.narg(scope_agent_id)::uuid IS NULL
       OR EXISTS (SELECT 1 FROM client_agents ca
                  WHERE ca.client_id = t.client_id AND ca.user_id = sqlc.narg(scope_agent_id)::uuid));

-- name: GetTicketForUpdate :one
SELECT t.id, t.key, t.type, t.status, t.priority, t.client_id, t.requester_id, t.assignee_id,
       a.name AS assignee_name, t.first_response_at
FROM tickets t
LEFT JOIN users a ON a.id = t.assignee_id
WHERE t.key = @key
  AND (sqlc.narg(scope_client_id)::uuid IS NULL OR t.client_id = sqlc.narg(scope_client_id)::uuid)
  AND (sqlc.narg(scope_agent_id)::uuid IS NULL
       OR EXISTS (SELECT 1 FROM client_agents ca
                  WHERE ca.client_id = t.client_id AND ca.user_id = sqlc.narg(scope_agent_id)::uuid))
FOR UPDATE OF t;

-- name: UpdateTicketStatus :exec
-- resolved keeps the first resolve time; leaving resolved clears it (tickets_resolved_at).
UPDATE tickets
SET status = @status,
    resolved_at = CASE WHEN @resolved::boolean THEN coalesce(resolved_at, now()) END
WHERE id = @id;

-- name: UpdateTicketAssignee :exec
-- One statement, so the assignee CHECKs never see a half-done change.
UPDATE tickets SET assignee_id = sqlc.narg(assignee_id), status = @status WHERE id = @id;

-- name: UpdateTicketPriority :exec
UPDATE tickets SET priority = @priority WHERE id = @id;

-- name: MarkTicketFirstResponse :execrows
UPDATE tickets SET first_response_at = now() WHERE id = @id AND first_response_at IS NULL;

-- name: TouchTicket :exec
-- Public activity only: requesters see updated_at, so internal notes must not move it.
UPDATE tickets SET updated_at = now() WHERE id = @id;

-- name: CreateTicketMessage :one
INSERT INTO ticket_messages (ticket_id, author_id, visibility, body)
VALUES (@ticket_id, @author_id, @visibility, @body)
RETURNING id, created_at;

-- name: ListTicketMessages :many
-- visibility NULL lists every message; requesters pass 'public'.
SELECT m.id, m.visibility, m.body, m.created_at, m.author_id, u.name AS author_name,
       u.role AS author_role, u.job_title AS author_job_title
FROM ticket_messages m
JOIN users u ON u.id = m.author_id
WHERE m.ticket_id = @ticket_id
  AND (sqlc.narg(visibility)::text IS NULL OR m.visibility = sqlc.narg(visibility)::text)
ORDER BY m.created_at, m.id;

-- name: InsertSeedTicket :one
-- Seed only: explicit number, status and times. Returns no row if the key exists.
INSERT INTO tickets (number, key, type, status, priority, channel, client_id, requester_id, assignee_id,
                     created_by, project_id, catalog_item_id, subject, description, sla_snapshot,
                     first_response_at, resolved_at, created_at, updated_at)
VALUES (@number, @key, @type, @status, @priority, @channel, @client_id, @requester_id, sqlc.narg(assignee_id),
        @created_by, @project_id, sqlc.narg(catalog_item_id), @subject, @description, @sla_snapshot,
        sqlc.narg(first_response_at), sqlc.narg(resolved_at), @created_at, @updated_at)
ON CONFLICT (key) DO NOTHING
RETURNING id;

-- name: InsertSeedMessage :exec
-- Seed only.
INSERT INTO ticket_messages (id, ticket_id, author_id, visibility, body, created_at)
VALUES (@id, @ticket_id, @author_id, @visibility, @body, @created_at);

-- name: SetTicketNumberFloor :exec
-- Seed only: continue numbering after the highest seeded ticket.
SELECT setval('ticket_number_seq', (SELECT max(number) FROM tickets));
