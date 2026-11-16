-- name: RecordDelivery :exec
-- A verified delivery arrived; the first one teaches the repository's own id.
UPDATE repositories
SET last_delivery_at = now(), external_id = coalesce(external_id, sqlc.narg(external_id))
WHERE id = @id;

-- name: RecordDeliveryError :exec
UPDATE repositories SET last_error = @last_error, last_error_at = now() WHERE id = @id;

-- name: InsertDelivery :one
-- Returns no row when the delivery was received before.
INSERT INTO webhook_deliveries (repository_id, delivery_id, event_name, payload)
VALUES (@repository_id, @delivery_id, @event_name, @payload)
ON CONFLICT (repository_id, delivery_id) DO NOTHING
RETURNING id;

-- name: GetDelivery :one
SELECT d.id, d.repository_id, d.event_name, d.payload, d.processed_at, r.provider
FROM webhook_deliveries d
JOIN repositories r ON r.id = d.repository_id
WHERE d.id = @id;

-- name: MarkDeliveryProcessed :exec
UPDATE webhook_deliveries SET processed_at = now() WHERE id = @id;

-- name: DeleteOldDeliveries :execrows
DELETE FROM webhook_deliveries
WHERE id IN (SELECT o.id FROM webhook_deliveries o WHERE o.received_at < @received_before
             ORDER BY o.received_at LIMIT @batch);

-- name: ListTicketPrefixes :many
-- Every prefix a ticket key can have: the current one and any older one still on a ticket.
SELECT ticket_prefix::text AS prefix FROM organization
UNION
SELECT DISTINCT split_part(key, '-', 1) FROM tickets;

-- name: FindStaffByEmail :one
-- The active staff member who writes code under this email.
SELECT id, name FROM users
WHERE email = lower(@email) AND role <> 'requester' AND status = 'active';

-- name: UpsertCodeEvent :one
-- A redelivered event keeps its row. A pull/merge request edited later takes the new title and
-- time, so its state reads from the latest activity.
INSERT INTO code_events (repository_id, kind, external_id, sha, number, title, branch, target, url,
                         author_name, author_login, author_user_id, delivery_id, occurred_at)
VALUES (@repository_id, @kind, @external_id, @sha, sqlc.narg(number), @title, @branch, @target, @url,
        @author_name, @author_login, sqlc.narg(author_user_id), sqlc.narg(delivery_id), @occurred_at)
ON CONFLICT (repository_id, kind, external_id) DO UPDATE
SET title = CASE WHEN code_events.kind = 'pr_opened' THEN EXCLUDED.title ELSE code_events.title END,
    occurred_at = CASE WHEN code_events.kind = 'pr_opened'
                       THEN greatest(code_events.occurred_at, EXCLUDED.occurred_at)
                       ELSE code_events.occurred_at END
RETURNING id;
