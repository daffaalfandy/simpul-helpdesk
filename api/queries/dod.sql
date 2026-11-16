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
