-- name: InsertAudit :exec
INSERT INTO audit_log (actor_kind, actor_user_id, actor_name, action, target_type, target_id,
                       data, reason, ip, request_id)
VALUES (@actor_kind, sqlc.narg(actor_user_id), sqlc.narg(actor_name), @action,
        sqlc.narg(target_type), sqlc.narg(target_id), @data, sqlc.narg(reason), sqlc.narg(ip),
        sqlc.narg(request_id));

-- name: ListAuditByTarget :many
-- Oldest first. Entries of one transaction share "at"; uuidv7 ids keep their insert order.
SELECT id, at, actor_kind, actor_user_id, actor_name, action, data, reason
FROM audit_log
WHERE target_type = @target_type AND target_id = @target_id
ORDER BY at, id;

-- name: InsertAuditAt :exec
-- Seed only: history with explicit times.
INSERT INTO audit_log (at, actor_kind, actor_user_id, actor_name, action, target_type, target_id, data, reason)
VALUES (@at, @actor_kind, sqlc.narg(actor_user_id), sqlc.narg(actor_name), @action, @target_type,
        @target_id, @data, sqlc.narg(reason));
