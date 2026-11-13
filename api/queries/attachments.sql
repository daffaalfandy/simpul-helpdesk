-- name: CreateAttachment :one
-- A pending upload: no ticket until LinkAttachments.
INSERT INTO attachments (storage_key, filename, content_type, size_bytes, sha256, uploaded_by)
VALUES (@storage_key, @filename, @content_type, @size_bytes, @sha256, @uploaded_by)
RETURNING id, created_at;

-- name: CountPendingAttachments :one
-- Unexpired pending uploads of one user.
SELECT count(*)::int FROM attachments
WHERE uploaded_by = @uploaded_by AND ticket_id IS NULL AND created_at > @not_before;

-- name: LinkAttachments :many
-- Only the uploader's own, still pending, unexpired files. A concurrent link or sweep makes
-- fewer rows come back, which the caller treats as an error.
UPDATE attachments
SET ticket_id = @ticket_id, message_id = sqlc.narg(message_id), linked_at = now()
WHERE id = ANY(@ids::uuid[])
  AND uploaded_by = @uploaded_by
  AND ticket_id IS NULL
  AND created_at > @not_before
RETURNING id, filename, content_type, size_bytes, created_at;

-- name: ListTicketAttachments :many
-- visibility 'public' hides files on internal notes; description files are always public.
SELECT a.id, a.message_id, a.filename, a.content_type, a.size_bytes, a.created_at
FROM attachments a
LEFT JOIN ticket_messages m ON m.id = a.message_id
WHERE a.ticket_id = @ticket_id
  AND (sqlc.narg(visibility)::text IS NULL OR a.message_id IS NULL OR m.visibility = sqlc.narg(visibility)::text)
ORDER BY a.created_at, a.id;

-- name: GetAttachmentForDownload :one
-- Scoped like GetTicket. Pending files never match: they have no ticket yet.
SELECT a.id, a.storage_key, a.filename, a.content_type, a.size_bytes
FROM attachments a
JOIN tickets t ON t.id = a.ticket_id
LEFT JOIN ticket_messages m ON m.id = a.message_id
WHERE a.id = @id
  AND (sqlc.narg(visibility)::text IS NULL OR a.message_id IS NULL OR m.visibility = sqlc.narg(visibility)::text)
  AND (sqlc.narg(scope_client_id)::uuid IS NULL OR t.client_id = sqlc.narg(scope_client_id)::uuid)
  AND (sqlc.narg(scope_agent_id)::uuid IS NULL
       OR EXISTS (SELECT 1 FROM client_agents ca
                  WHERE ca.client_id = t.client_id AND ca.user_id = sqlc.narg(scope_agent_id)::uuid));

-- name: ListExpiredPendingAttachments :many
SELECT id, storage_key FROM attachments
WHERE ticket_id IS NULL AND created_at < @cutoff
ORDER BY created_at
LIMIT @batch_size
FOR UPDATE SKIP LOCKED;

-- name: DeleteAttachments :exec
DELETE FROM attachments WHERE id = ANY(@ids::uuid[]) AND ticket_id IS NULL;
