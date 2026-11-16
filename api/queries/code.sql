-- name: ListTicketsByKeys :many
-- The tickets a delivery names, with the repository their project reads code from.
SELECT t.id, t.key, p.repository_id
FROM tickets t
JOIN projects p ON p.id = t.project_id
WHERE t.key = ANY(@keys::text[]);

-- name: ListRunCandidates :many
-- Open tickets a CI run or deployment in the repository may complete: their project reads code
-- from it and code already reached them.
SELECT t.id, t.key, p.dod_config, o.config AS dod_override
FROM tickets t
JOIN projects p ON p.id = t.project_id
LEFT JOIN ticket_dod_overrides o ON o.ticket_id = t.id
WHERE p.repository_id = @repository_id AND t.status = ANY(@statuses::text[])
  AND EXISTS (SELECT 1 FROM code_event_links l WHERE l.ticket_id = t.id
              AND l.outcome NOT IN ('ticket_closed', 'unknown_ticket', 'other_repository'))
ORDER BY t.id;

-- name: ListRunsSince :many
-- Successful CI runs and deployments stored for the repository from @since: one read before
-- the merge it follows still counts.
SELECT e.id, e.kind, e.sha, e.number, e.title, e.branch, e.target, e.occurred_at, e.author_user_id,
       e.author_login, e.author_name, u.name AS author_user_name
FROM code_events e
LEFT JOIN users u ON u.id = e.author_user_id
WHERE e.repository_id = @repository_id AND e.kind IN ('pipeline', 'deployment') AND e.occurred_at >= @since
ORDER BY e.occurred_at, e.id
LIMIT @row_limit;

-- name: InsertCodeLink :exec
-- An ignored key is logged once per event (code_event_links_ignored_key).
INSERT INTO code_event_links (code_event_id, ticket_id, ticket_key, outcome, items, status_from, status_to)
VALUES (@code_event_id, sqlc.narg(ticket_id), @ticket_key, @outcome, @items::text[], sqlc.narg(status_from), sqlc.narg(status_to))
ON CONFLICT DO NOTHING;

-- name: GetTicketForCode :one
SELECT t.id, t.key, t.status, t.assignee_id, a.name AS assignee_name, t.client_id, p.repository_id,
       p.dod_config, o.config AS dod_override
FROM tickets t
JOIN projects p ON p.id = t.project_id
LEFT JOIN users a ON a.id = t.assignee_id
LEFT JOIN ticket_dod_overrides o ON o.ticket_id = t.id
WHERE t.id = @id
FOR UPDATE OF t;

-- name: ListTicketCodeLinks :many
-- The code events that reached a ticket, ignored or not, with when they happened.
SELECT l.code_event_id, e.occurred_at, l.outcome
FROM code_event_links l
JOIN code_events e ON e.id = l.code_event_id
WHERE l.ticket_id = @ticket_id;

-- name: ListTicketDoDItemTimes :many
-- Items a ticket has met, dated by the code event that met them (a merge's own time) or else
-- when they were recorded.
SELECT i.item, coalesce(e.occurred_at, i.done_at)::timestamptz AS at
FROM ticket_dod_items i
LEFT JOIN code_events e ON e.id = i.code_event_id
WHERE i.ticket_id = @ticket_id;
