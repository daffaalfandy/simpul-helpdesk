-- name: ListTicketDoDItems :many
SELECT i.item, i.done_at, i.done_by, u.name AS done_by_name
FROM ticket_dod_items i
LEFT JOIN users u ON u.id = i.done_by
WHERE i.ticket_id = @ticket_id
ORDER BY i.done_at, i.item;

-- name: InsertTicketDoDItems :many
-- Returns the items newly met; items already met keep their first time and person.
INSERT INTO ticket_dod_items (ticket_id, item, done_by)
SELECT @ticket_id, unnest(@items::text[]), sqlc.narg(done_by)::uuid
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
-- oldest first, with the team's latest public reply as the test instructions ('' when the
-- team hasn't written one; message bodies are never empty).
SELECT t.id, t.key, t.subject, t.status_changed_at, t.project_id, p.name AS project_name,
       p.dod_config, o.config AS dod_override, t.requester_id, r.name AS requester_name,
       coalesce(last.body, '')::text AS instructions
FROM tickets t
JOIN projects p ON p.id = t.project_id
JOIN users r ON r.id = t.requester_id
LEFT JOIN ticket_dod_overrides o ON o.ticket_id = t.id
LEFT JOIN LATERAL (
  SELECT m.body FROM ticket_messages m JOIN users u ON u.id = m.author_id
  WHERE m.ticket_id = t.id AND m.visibility = @public_visibility AND u.role <> @requester_role
  ORDER BY m.created_at DESC, m.id DESC LIMIT 1
) last ON true
WHERE t.client_id = @client_id AND t.status = @status
  AND (t.requester_id = @user_id
       OR EXISTS (SELECT 1 FROM ticket_watchers w WHERE w.ticket_id = t.id AND w.user_id = @user_id))
ORDER BY t.status_changed_at, t.id
LIMIT @row_limit;

-- name: GetLatestPublicStaffMessage :one
-- The team's latest public reply on a ticket: what the client is asked to test.
SELECT m.body FROM ticket_messages m JOIN users u ON u.id = m.author_id
WHERE m.ticket_id = @ticket_id AND m.visibility = @public_visibility AND u.role <> @requester_role
ORDER BY m.created_at DESC, m.id DESC
LIMIT 1;
