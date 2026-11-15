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
-- created_at is when the change happened, not when the fan-out ran.
INSERT INTO notifications (user_id, ticket_id, event_type, actor_user_id, source_id, message_id, data,
                           web, email, urgent, created_at)
VALUES (@user_id, @ticket_id, @event_type, sqlc.narg(actor_user_id), @source_id, sqlc.narg(message_id),
        @data, @web, @email, @urgent, @created_at)
ON CONFLICT (source_id, user_id) DO NOTHING;

-- name: GetLatestPublicStaffMessageID :one
-- The team's latest public reply: what a "question for you" quotes.
SELECT m.id
FROM ticket_messages m
JOIN users u ON u.id = m.author_id
WHERE m.ticket_id = @ticket_id AND m.visibility = @visibility AND u.role <> @requester_role
ORDER BY m.created_at DESC, m.id DESC
LIMIT 1;

-- Inbox queries are scoped like tickets.sql: scope_client_id and scope_agent_id come from
-- internal/tickets access rules (tickets.ScopeFor), so a user stops seeing items about tickets
-- they lose access to.

-- name: ListNotifications :many
-- Newest first; before pages on (created_at, id). The quote is cut to quote_runes characters;
-- the service shows it only for public messages unless the user is staff.
SELECT n.id, n.event_type, n.data, n.created_at, n.read_at, n.web, n.email,
       t.key AS ticket_key, t.subject AS ticket_subject, a.name AS actor_name,
       m.visibility AS quote_visibility, coalesce(left(m.body, @quote_runes::int), '')::text AS quote
FROM notifications n
JOIN tickets t ON t.id = n.ticket_id
LEFT JOIN users a ON a.id = n.actor_user_id
LEFT JOIN ticket_messages m ON m.id = n.message_id
WHERE n.user_id = @user_id AND n.web
  AND (sqlc.narg(scope_client_id)::uuid IS NULL OR t.client_id = sqlc.narg(scope_client_id)::uuid)
  AND (sqlc.narg(scope_agent_id)::uuid IS NULL
       OR EXISTS (SELECT 1 FROM client_agents ca
                  WHERE ca.client_id = t.client_id AND ca.user_id = sqlc.narg(scope_agent_id)::uuid))
  AND (cardinality(@events::text[]) = 0 OR n.event_type = ANY(@events::text[]))
  AND (NOT @unread_only::boolean OR n.read_at IS NULL)
  AND (sqlc.narg(before)::uuid IS NULL
       OR (n.created_at, n.id) < (SELECT b.created_at, b.id FROM notifications b
                                  WHERE b.id = sqlc.narg(before)::uuid AND b.user_id = @user_id))
ORDER BY n.created_at DESC, n.id DESC
LIMIT @row_limit;

-- name: CountUnreadNotifications :one
SELECT count(*)::int
FROM notifications n
JOIN tickets t ON t.id = n.ticket_id
WHERE n.user_id = @user_id AND n.web AND n.read_at IS NULL
  AND (sqlc.narg(scope_client_id)::uuid IS NULL OR t.client_id = sqlc.narg(scope_client_id)::uuid)
  AND (sqlc.narg(scope_agent_id)::uuid IS NULL
       OR EXISTS (SELECT 1 FROM client_agents ca
                  WHERE ca.client_id = t.client_id AND ca.user_id = sqlc.narg(scope_agent_id)::uuid));

-- name: SetNotificationRead :execrows
UPDATE notifications n
SET read_at = CASE WHEN @read::boolean THEN coalesce(n.read_at, now()) END
FROM tickets t
WHERE n.id = @id AND n.user_id = @user_id AND n.web AND t.id = n.ticket_id
  AND (sqlc.narg(scope_client_id)::uuid IS NULL OR t.client_id = sqlc.narg(scope_client_id)::uuid)
  AND (sqlc.narg(scope_agent_id)::uuid IS NULL
       OR EXISTS (SELECT 1 FROM client_agents ca
                  WHERE ca.client_id = t.client_id AND ca.user_id = sqlc.narg(scope_agent_id)::uuid));

-- name: MarkAllNotificationsRead :exec
UPDATE notifications n
SET read_at = now()
FROM tickets t
WHERE n.user_id = @user_id AND n.web AND n.read_at IS NULL AND t.id = n.ticket_id
  AND (sqlc.narg(scope_client_id)::uuid IS NULL OR t.client_id = sqlc.narg(scope_client_id)::uuid)
  AND (sqlc.narg(scope_agent_id)::uuid IS NULL
       OR EXISTS (SELECT 1 FROM client_agents ca
                  WHERE ca.client_id = t.client_id AND ca.user_id = sqlc.narg(scope_agent_id)::uuid));

-- name: GetUserEmailFrequency :one
SELECT email_frequency FROM users WHERE id = @id;

-- name: SetUserEmailFrequency :exec
UPDATE users SET email_frequency = @email_frequency WHERE id = @id;

-- name: UpsertNotificationPreference :exec
INSERT INTO notification_preferences (user_id, event_type, web, email)
VALUES (@user_id, @event_type, @web, @email)
ON CONFLICT (user_id, event_type) DO UPDATE
SET web = EXCLUDED.web, email = EXCLUDED.email, updated_at = now();

-- Email delivery. A send job claims rows (email_claimed_at), sends, then marks them emailed. A
-- claim older than stale_before is abandoned (a crashed job) and the rows are pending again.

-- name: ListPendingEmails :many
SELECT n.id, n.user_id, n.ticket_id, n.urgent, n.created_at, u.email_frequency
FROM notifications n
JOIN users u ON u.id = n.user_id
WHERE n.email AND n.emailed_at IS NULL
  AND (n.email_claimed_at IS NULL OR n.email_claimed_at < @stale_before::timestamptz)
ORDER BY n.created_at, n.id
LIMIT @row_limit;

-- name: ClaimTicketEmail :many
-- One user's pending rows about one ticket: all of them, or only urgent ones for summary users.
UPDATE notifications SET email_claimed_at = now()
WHERE id IN (
  SELECT n.id FROM notifications n
  WHERE n.user_id = @user_id AND n.ticket_id = @ticket_id AND n.email AND n.emailed_at IS NULL
    AND (n.email_claimed_at IS NULL OR n.email_claimed_at < @stale_before::timestamptz)
    AND (n.urgent OR @include_routine::boolean)
  FOR UPDATE SKIP LOCKED)
RETURNING id, ticket_id, event_type, actor_user_id, message_id, data, urgent, created_at;

-- name: ClaimDigest :many
-- One user's routine pending rows created before the summary boundary.
UPDATE notifications SET email_claimed_at = now()
WHERE id IN (
  SELECT n.id FROM notifications n
  WHERE n.user_id = @user_id AND n.email AND n.emailed_at IS NULL AND NOT n.urgent
    AND n.created_at < @created_before
    AND (n.email_claimed_at IS NULL OR n.email_claimed_at < @stale_before::timestamptz)
  FOR UPDATE SKIP LOCKED)
RETURNING id, ticket_id, event_type, actor_user_id, message_id, data, urgent, created_at;

-- name: MarkNotificationsEmailed :exec
UPDATE notifications SET emailed_at = now(), email_skipped = @skipped WHERE id = ANY(@ids::uuid[]);

-- name: ReleaseNotificationClaims :exec
-- A failed send gives its rows back for the retry.
UPDATE notifications SET email_claimed_at = NULL WHERE id = ANY(@ids::uuid[]) AND emailed_at IS NULL;

-- name: GetEmailRecipient :one
SELECT id, email, name, role, status, client_id, email_frequency FROM users WHERE id = @id;

-- name: ListEmailTickets :many
-- What an email shows about each ticket, and whether the recipient may still see it (the same
-- rule as internal/tickets access: all for admins and supervisors, linked clients for agents,
-- their own client for requesters).
SELECT t.id, t.key, t.subject, t.priority, t.requester_id, t.assignee_id, t.sla_snapshot,
       pl.name AS priority_name, p.name AS project_name,
       (u.role = ANY(@all_client_roles::text[])
        OR (u.role = @agent_role AND EXISTS (SELECT 1 FROM client_agents ca
                                            WHERE ca.client_id = t.client_id AND ca.user_id = u.id))
        OR (u.role = @requester_role AND u.client_id = t.client_id))::boolean AS visible,
       EXISTS (SELECT 1 FROM ticket_watchers w WHERE w.ticket_id = t.id AND w.user_id = u.id) AS watching
FROM tickets t
JOIN projects p ON p.id = t.project_id
JOIN priorities pl ON pl.code = t.priority
JOIN users u ON u.id = @user_id
WHERE t.id = ANY(@ticket_ids::uuid[]);

-- name: ListUserNames :many
SELECT id, name FROM users WHERE id = ANY(@ids::uuid[]);

-- name: GetEmailQuote :one
SELECT m.body, m.visibility, m.created_at, u.name AS author_name, u.role AS author_role
FROM ticket_messages m
JOIN users u ON u.id = m.author_id
WHERE m.id = @id;

-- name: DeleteOldNotifications :execrows
DELETE FROM notifications
WHERE id IN (SELECT o.id FROM notifications o WHERE o.created_at < @created_before
             ORDER BY o.created_at LIMIT @batch);

-- name: ListTicketIDsByNumber :many
SELECT id FROM tickets WHERE number = ANY(@numbers::bigint[]) ORDER BY number;

-- name: SettleSeedNotifications :exec
-- Demo history: nothing to email, and items older than read_before already read.
UPDATE notifications
SET emailed_at = coalesce(emailed_at, created_at),
    read_at = CASE WHEN created_at < @read_before THEN coalesce(read_at, created_at) ELSE read_at END
WHERE ticket_id = ANY(@ticket_ids::uuid[]);
