-- name: ListTicketsByKeys :many
-- The tickets a delivery names, with the repository their project reads code from.
SELECT t.id, t.key, p.repository_id
FROM tickets t
JOIN projects p ON p.id = t.project_id
WHERE t.key = ANY(@keys::text[]);

-- name: InsertCodeLink :exec
-- An ignored key is logged once per event (code_event_links_ignored_key).
INSERT INTO code_event_links (code_event_id, ticket_id, ticket_key, outcome)
VALUES (@code_event_id, sqlc.narg(ticket_id), @ticket_key, @outcome)
ON CONFLICT DO NOTHING;

-- name: GetTicketForCode :one
SELECT t.id, t.key, t.status, p.repository_id
FROM tickets t
JOIN projects p ON p.id = t.project_id
WHERE t.id = @id
FOR UPDATE OF t;

-- name: ListTicketCodeLinks :many
-- The code events that reached a ticket, ignored or not.
SELECT l.code_event_id
FROM code_event_links l
WHERE l.ticket_id = @ticket_id;

-- name: ListLinkedCodeEvents :many
-- The code events that reached the tickets (ignored keys left out), once per ticket and event.
SELECT DISTINCT ON (l.ticket_id, e.id)
       l.ticket_id, e.id, e.sha, e.title, e.branches, e.url, e.author_login,
       e.author_name, u.name AS author_user_name, e.occurred_at, r.provider
FROM code_event_links l
JOIN code_events e ON e.id = l.code_event_id
JOIN repositories r ON r.id = e.repository_id
LEFT JOIN users u ON u.id = e.author_user_id
WHERE l.ticket_id = ANY(@ticket_ids::uuid[])
  AND l.outcome NOT IN ('ticket_closed', 'unknown_ticket', 'other_repository')
ORDER BY l.ticket_id, e.id;

-- name: GetTicketRepository :one
-- The repository a ticket's project reads code from.
SELECT r.id, r.provider, r.full_name, r.web_url
FROM tickets t
JOIN projects p ON p.id = t.project_id
JOIN repositories r ON r.id = p.repository_id
WHERE t.id = @ticket_id;

-- name: ListCodeLog :many
-- The latest commits linked to tickets, ignored keys included, for the Integrations page.
SELECT l.id, l.at, l.ticket_key, (l.ticket_id IS NOT NULL)::boolean AS ticket_found, l.outcome,
       e.sha, e.title, e.branch, e.url, r.provider, r.full_name
FROM code_event_links l
JOIN code_events e ON e.id = l.code_event_id
JOIN repositories r ON r.id = e.repository_id
ORDER BY l.at DESC, l.id DESC
LIMIT @row_limit;

-- name: TicketKeysExist :many
SELECT key FROM tickets WHERE key = ANY(@keys::text[]);

-- name: InsertSeedCodeLink :exec
-- Seed only: a commit linked at an explicit time.
INSERT INTO code_event_links (code_event_id, ticket_id, ticket_key, outcome, at)
VALUES (@code_event_id, sqlc.narg(ticket_id), @ticket_key, @outcome, @at)
ON CONFLICT DO NOTHING;
