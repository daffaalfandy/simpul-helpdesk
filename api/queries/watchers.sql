-- name: ListTicketWatchers :many
SELECT u.id, u.name, u.job_title
FROM ticket_watchers w
JOIN users u ON u.id = w.user_id
WHERE w.ticket_id = @ticket_id
ORDER BY lower(u.name), u.id;

-- name: ListTicketWatcherIDs :many
SELECT user_id FROM ticket_watchers WHERE ticket_id = @ticket_id;

-- name: ListWatcherCandidates :many
-- The client's other requesters who could follow the ticket: not its requester, not already
-- following, not suspended. pattern is an escaped ILIKE pattern over the name, or NULL.
SELECT u.id, u.name, u.job_title
FROM users u
WHERE u.client_id = @client_id
  AND u.role = @role
  AND u.status = ANY(@statuses::text[])
  AND u.id <> @requester_id
  AND NOT EXISTS (SELECT 1 FROM ticket_watchers w WHERE w.ticket_id = @ticket_id AND w.user_id = u.id)
  AND (sqlc.narg(pattern)::text IS NULL OR u.name ILIKE sqlc.narg(pattern)::text)
ORDER BY lower(u.name), u.id
LIMIT @row_limit;

-- name: AddTicketWatcher :execrows
INSERT INTO ticket_watchers (ticket_id, user_id, added_by)
VALUES (@ticket_id, @user_id, @added_by)
ON CONFLICT DO NOTHING;

-- name: RemoveTicketWatcher :execrows
DELETE FROM ticket_watchers WHERE ticket_id = @ticket_id AND user_id = @user_id;
