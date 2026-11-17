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
SELECT d.id, d.repository_id, d.event_name, d.payload, d.processed_at, r.provider, r.organization_id
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
-- Every prefix an organization's ticket keys can have: the current one and any older one still
-- on one of its tickets.
SELECT o.ticket_prefix::text AS prefix FROM organizations o WHERE o.id = @organization_id::uuid
UNION
SELECT DISTINCT split_part(t.key, '-', 1) FROM tickets t WHERE t.organization_id = @organization_id::uuid;

-- name: FindStaffByEmail :one
-- The active administrator or member of the organization who writes code under this email.
SELECT u.id, u.name FROM users u
WHERE u.email = lower(@email) AND u.status = @status
  AND (u.role = @admin_role
       OR EXISTS (SELECT 1 FROM organization_members m WHERE m.user_id = u.id AND m.organization_id = @organization_id));

-- name: UpsertCodeEvent :one
-- A commit pushed again to another branch (the merge into main) keeps its row and adds the
-- branch; a redelivery adds nothing.
INSERT INTO code_events (repository_id, kind, external_id, sha, title, branch, branches, url,
                         author_name, author_login, author_user_id, delivery_id, occurred_at)
VALUES (@repository_id, @kind, @external_id, @sha, @title, @branch, ARRAY[@branch::text], @url,
        @author_name, @author_login, sqlc.narg(author_user_id), sqlc.narg(delivery_id), @occurred_at)
ON CONFLICT (repository_id, kind, external_id) DO UPDATE
SET branches = CASE WHEN EXCLUDED.branch = ANY(code_events.branches) THEN code_events.branches
                    ELSE code_events.branches || EXCLUDED.branch END
RETURNING id;
