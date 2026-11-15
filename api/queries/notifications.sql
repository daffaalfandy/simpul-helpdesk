-- name: GetTicketForFanout :one
SELECT t.id, t.priority, t.requester_id, t.assignee_id, t.sla_snapshot
FROM tickets t
WHERE t.id = @id;

-- name: ListNotifyRecipients :many
-- What the router needs about each user a change may concern.
SELECT id, role, status FROM users WHERE id = ANY(@ids::uuid[]);

-- name: ListActiveUserIDsByRole :many
SELECT id FROM users WHERE role = @role AND status = @status ORDER BY id;

-- name: ListNotificationPreferences :many
SELECT user_id, event_type, web, email
FROM notification_preferences
WHERE user_id = ANY(@user_ids::uuid[]);

-- name: InsertNotification :exec
-- source_id + user_id is unique, so a retried fan-out adds nothing.
INSERT INTO notifications (user_id, ticket_id, event_type, actor_user_id, source_id, message_id, data,
                           web, email, urgent)
VALUES (@user_id, @ticket_id, @event_type, sqlc.narg(actor_user_id), @source_id, sqlc.narg(message_id),
        @data, @web, @email, @urgent)
ON CONFLICT (source_id, user_id) DO NOTHING;
