-- name: ListTicketDoDItems :many
-- With the code event that met an item, if one did.
SELECT i.item, i.done_at, i.done_by, u.name AS done_by_name, e.id AS code_event_id, e.kind AS code_kind,
       e.sha AS code_sha, e.number AS code_number, e.title AS code_title, e.branch AS code_branch,
       e.target AS code_target, e.url AS code_url, e.author_login AS code_author_login,
       e.author_name AS code_author_name, e.occurred_at AS code_at, r.provider AS code_provider
FROM ticket_dod_items i
LEFT JOIN users u ON u.id = i.done_by
LEFT JOIN code_events e ON e.id = i.code_event_id
LEFT JOIN repositories r ON r.id = e.repository_id
WHERE i.ticket_id = @ticket_id
ORDER BY i.done_at, i.item;

-- name: InsertTicketDoDItems :many
-- Returns the items newly met; items already met keep their first time and person. A code
-- item names the code event that met it.
INSERT INTO ticket_dod_items (ticket_id, item, done_by, code_event_id)
SELECT @ticket_id, unnest(@items::text[]), sqlc.narg(done_by)::uuid, sqlc.narg(code_event_id)::uuid
ON CONFLICT (ticket_id, item) DO NOTHING
RETURNING item;

-- name: DeleteTicketDoDItems :many
DELETE FROM ticket_dod_items WHERE ticket_id = @ticket_id AND item = ANY(@items::text[])
RETURNING item;

-- name: UpsertTicketDoDOverride :exec
INSERT INTO ticket_dod_overrides (ticket_id, config, reason, set_by)
VALUES (@ticket_id, @config, @reason, @set_by)
ON CONFLICT (ticket_id) DO UPDATE
SET config = EXCLUDED.config, reason = EXCLUDED.reason, set_by = EXCLUDED.set_by, set_at = now();

-- name: DeleteTicketDoDOverride :execrows
DELETE FROM ticket_dod_overrides WHERE ticket_id = @ticket_id;

-- name: ListUATRequests :many
-- The tests waiting on a requester: their client's tickets In UAT that they reported or follow,
-- oldest first.
SELECT t.id, t.key, t.subject, t.status_changed_at, t.project_id, p.name AS project_name,
       p.dod_config, o.config AS dod_override, t.requester_id, r.name AS requester_name
FROM tickets t
JOIN projects p ON p.id = t.project_id
JOIN users r ON r.id = t.requester_id
LEFT JOIN ticket_dod_overrides o ON o.ticket_id = t.id
WHERE t.client_id = @client_id AND t.status = @status
  AND (t.requester_id = @user_id
       OR EXISTS (SELECT 1 FROM ticket_watchers w WHERE w.ticket_id = t.id AND w.user_id = @user_id))
ORDER BY t.status_changed_at, t.id
LIMIT @row_limit;

-- name: ListLifecycleCandidates :many
-- Tickets that may be past a client deadline: In UAT or resolved since before @changed_before
-- (a deadline is at least one working day of business time, which never passes faster than
-- real time), with what the deadline is computed from. Read without locks; keyset pages by id.
SELECT t.id, t.status, t.resolved_at, t.status_changed_at, p.dod_config, o.config AS dod_override
FROM tickets t
JOIN projects p ON p.id = t.project_id
LEFT JOIN ticket_dod_overrides o ON o.ticket_id = t.id
WHERE t.status = ANY(@statuses::text[]) AND t.status_changed_at <= @changed_before
  AND (sqlc.narg(after)::uuid IS NULL OR t.id > sqlc.narg(after)::uuid)
ORDER BY t.id
LIMIT @row_limit;

-- name: GetTicketForLifecycle :one
SELECT t.id, t.status, t.assignee_id, t.resolved_at, t.status_changed_at, p.dod_config, o.config AS dod_override
FROM tickets t
JOIN projects p ON p.id = t.project_id
LEFT JOIN ticket_dod_overrides o ON o.ticket_id = t.id
WHERE t.id = @id
FOR UPDATE OF t;

-- name: InsertSeedDoDItem :exec
-- Seed only: an item met at an explicit time.
INSERT INTO ticket_dod_items (ticket_id, item, done_at, done_by)
VALUES (@ticket_id, @item, @done_at, sqlc.narg(done_by))
ON CONFLICT (ticket_id, item) DO NOTHING;
