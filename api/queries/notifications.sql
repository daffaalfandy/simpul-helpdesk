-- name: GetTicketForFanout :one
SELECT t.id, t.organization_id, t.priority, t.requester_id, t.assignee_id, t.sla_snapshot, p.dod_config,
       o.config AS dod_override
FROM tickets t
JOIN projects p ON p.id = t.project_id
LEFT JOIN ticket_dod_overrides o ON o.ticket_id = t.id
WHERE t.id = @id;

-- name: ListNotifyRecipients :many
-- What the router needs about each user a change may concern, and whether they still work in
-- the ticket's organization (administrators always do; staff while they are members).
SELECT u.id, u.role, u.status,
       (u.role <> @staff_role
        OR EXISTS (SELECT 1 FROM organization_members m
                   WHERE m.user_id = u.id AND m.organization_id = @organization_id))::boolean AS in_organization
FROM users u WHERE u.id = ANY(@ids::uuid[]);

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

-- Inbox queries are scoped like tickets.sql: scope_client_id, scope_requester_id and
-- scope_agent_id come from internal/tickets access rules (tickets.ScopeFor), so a user stops
-- seeing items about tickets they lose access to.

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
  AND t.organization_id = @organization_id
  AND (sqlc.narg(scope_client_id)::uuid IS NULL OR t.client_id = sqlc.narg(scope_client_id)::uuid)
  AND (sqlc.narg(scope_requester_id)::uuid IS NULL
       OR t.requester_id = sqlc.narg(scope_requester_id)::uuid
       OR EXISTS (SELECT 1 FROM requester_projects rp
                  WHERE rp.user_id = sqlc.narg(scope_requester_id)::uuid AND rp.project_id = t.project_id))
  AND (sqlc.narg(scope_agent_id)::uuid IS NULL
       OR EXISTS (SELECT 1 FROM client_agents ca
                  WHERE ca.client_id = t.client_id AND ca.user_id = sqlc.narg(scope_agent_id)::uuid)
       OR t.assignee_id = sqlc.narg(scope_agent_id)::uuid)
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
  AND t.organization_id = @organization_id
  AND (sqlc.narg(scope_client_id)::uuid IS NULL OR t.client_id = sqlc.narg(scope_client_id)::uuid)
  AND (sqlc.narg(scope_requester_id)::uuid IS NULL
       OR t.requester_id = sqlc.narg(scope_requester_id)::uuid
       OR EXISTS (SELECT 1 FROM requester_projects rp
                  WHERE rp.user_id = sqlc.narg(scope_requester_id)::uuid AND rp.project_id = t.project_id))
  AND (sqlc.narg(scope_agent_id)::uuid IS NULL
       OR EXISTS (SELECT 1 FROM client_agents ca
                  WHERE ca.client_id = t.client_id AND ca.user_id = sqlc.narg(scope_agent_id)::uuid)
       OR t.assignee_id = sqlc.narg(scope_agent_id)::uuid);

-- name: SetNotificationRead :execrows
UPDATE notifications n
SET read_at = CASE WHEN @read::boolean THEN coalesce(n.read_at, now()) END
FROM tickets t
WHERE n.id = @id AND n.user_id = @user_id AND n.web AND t.id = n.ticket_id
  AND t.organization_id = @organization_id
  AND (sqlc.narg(scope_client_id)::uuid IS NULL OR t.client_id = sqlc.narg(scope_client_id)::uuid)
  AND (sqlc.narg(scope_requester_id)::uuid IS NULL
       OR t.requester_id = sqlc.narg(scope_requester_id)::uuid
       OR EXISTS (SELECT 1 FROM requester_projects rp
                  WHERE rp.user_id = sqlc.narg(scope_requester_id)::uuid AND rp.project_id = t.project_id))
  AND (sqlc.narg(scope_agent_id)::uuid IS NULL
       OR EXISTS (SELECT 1 FROM client_agents ca
                  WHERE ca.client_id = t.client_id AND ca.user_id = sqlc.narg(scope_agent_id)::uuid)
       OR t.assignee_id = sqlc.narg(scope_agent_id)::uuid);

-- name: MarkAllNotificationsRead :exec
UPDATE notifications n
SET read_at = now()
FROM tickets t
WHERE n.user_id = @user_id AND n.web AND n.read_at IS NULL AND t.id = n.ticket_id
  AND t.organization_id = @organization_id
  AND (sqlc.narg(scope_client_id)::uuid IS NULL OR t.client_id = sqlc.narg(scope_client_id)::uuid)
  AND (sqlc.narg(scope_requester_id)::uuid IS NULL
       OR t.requester_id = sqlc.narg(scope_requester_id)::uuid
       OR EXISTS (SELECT 1 FROM requester_projects rp
                  WHERE rp.user_id = sqlc.narg(scope_requester_id)::uuid AND rp.project_id = t.project_id))
  AND (sqlc.narg(scope_agent_id)::uuid IS NULL
       OR EXISTS (SELECT 1 FROM client_agents ca
                  WHERE ca.client_id = t.client_id AND ca.user_id = sqlc.narg(scope_agent_id)::uuid)
       OR t.assignee_id = sqlc.narg(scope_agent_id)::uuid);

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

-- name: ListDueEmails :many
-- Pending rows whose email is due: ticket emails (instant users, and urgent rows for everyone)
-- once the row has gathered for the group window, summary rows from before their user's last
-- boundary. Rows not yet due stay out, so they can never crowd due ones out of the batch.
-- One organization at a time: summary boundaries follow its time zone.
SELECT n.id, n.user_id, n.ticket_id, n.urgent, n.created_at, u.email_frequency
FROM notifications n
JOIN users u ON u.id = n.user_id
JOIN tickets t ON t.id = n.ticket_id
WHERE t.organization_id = @organization_id
  AND n.email AND n.emailed_at IS NULL
  AND (n.email_claimed_at IS NULL OR n.email_claimed_at < @stale_before::timestamptz)
  AND (((n.urgent OR u.email_frequency = @instant::text) AND n.created_at <= @group_cutoff::timestamptz)
    OR (NOT n.urgent AND u.email_frequency = @hourly::text AND n.created_at < @hourly_cutoff::timestamptz)
    OR (NOT n.urgent AND u.email_frequency = @daily::text AND n.created_at < @daily_cutoff::timestamptz))
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
-- One user's routine pending rows created before the summary boundary, about one organization's
-- tickets (NULL: every organization, for summaries queued before organizations existed).
UPDATE notifications SET email_claimed_at = now()
WHERE id IN (
  SELECT n.id FROM notifications n
  WHERE n.user_id = @user_id AND n.email AND n.emailed_at IS NULL AND NOT n.urgent
    AND (sqlc.narg(organization_id)::uuid IS NULL
         OR n.ticket_id IN (SELECT t.id FROM tickets t WHERE t.organization_id = sqlc.narg(organization_id)::uuid))
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
-- rule as internal/tickets access: all for administrators and the organization's supervisors,
-- linked clients and tickets assigned to them for its agents, and for requesters the tickets of
-- their client they reported or that belong to a project they are linked to).
-- member_role is the recipient's role in the ticket's organization (NULL: not a member).
SELECT t.id, t.organization_id, t.key, t.subject, t.type, t.status, t.priority, t.requester_id, t.assignee_id,
       t.sla_snapshot, pl.name AS priority_name, p.name AS project_name, m.role AS member_role,
       -- coalesce: m.role is NULL for non-members, which would leave a refusal NULL.
       coalesce(u.role = @admin_role
                OR m.role = @supervisor_role
                OR (m.role = @agent_role AND (t.assignee_id = u.id
                                              OR EXISTS (SELECT 1 FROM client_agents ca
                                                         WHERE ca.client_id = t.client_id AND ca.user_id = u.id)))
                OR (u.role = @requester_role AND u.client_id = t.client_id
                    AND (t.requester_id = u.id
                         OR EXISTS (SELECT 1 FROM requester_projects rp
                                    WHERE rp.user_id = u.id AND rp.project_id = t.project_id))),
                false)::boolean AS visible,
       EXISTS (SELECT 1 FROM ticket_watchers w WHERE w.ticket_id = t.id AND w.user_id = u.id) AS watching
FROM tickets t
JOIN projects p ON p.id = t.project_id
JOIN priorities pl ON pl.organization_id = t.organization_id AND pl.code = t.priority
JOIN users u ON u.id = @user_id
LEFT JOIN organization_members m ON m.user_id = u.id AND m.organization_id = t.organization_id
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

-- name: GetPicEmailContext :one
-- What the client contact's breach email shows.
SELECT t.organization_id, t.key, t.subject, t.type, t.priority, t.sla_snapshot, pl.name AS priority_name,
       p.name AS project_name, c.name AS client_name, c.pic_name, c.pic_email, s.resolution_breached_at
FROM tickets t
JOIN projects p ON p.id = t.project_id
JOIN priorities pl ON pl.organization_id = t.organization_id AND pl.code = t.priority
JOIN clients c ON c.id = t.client_id
LEFT JOIN ticket_sla s ON s.ticket_id = t.id
WHERE t.id = @id;
