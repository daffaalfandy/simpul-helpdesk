-- name: InsertAudit :exec
INSERT INTO audit_log (actor_kind, actor_user_id, actor_name, action, target_type, target_id,
                       data, reason, ip, request_id)
VALUES (@actor_kind, sqlc.narg(actor_user_id), sqlc.narg(actor_name), @action,
        sqlc.narg(target_type), sqlc.narg(target_id), @data, sqlc.narg(reason), sqlc.narg(ip),
        sqlc.narg(request_id));
