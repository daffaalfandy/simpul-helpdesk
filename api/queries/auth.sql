-- name: CreateSession :one
INSERT INTO sessions (token_hash, user_id, expires_at, ip, user_agent)
VALUES (@token_hash, @user_id, @expires_at, sqlc.narg(ip), sqlc.narg(user_agent))
RETURNING id;

-- name: GetSessionPrincipal :one
SELECT s.id AS session_id, s.created_at AS session_created_at, s.last_seen_at, s.expires_at,
       u.id AS user_id, u.email, u.name, u.role, u.client_id, c.name AS client_name,
       u.job_title, u.status, u.password_changed_at
FROM sessions s
JOIN users u ON u.id = s.user_id
LEFT JOIN clients c ON c.id = u.client_id
WHERE s.token_hash = @token_hash;

-- name: TouchSession :exec
UPDATE sessions SET last_seen_at = now(), expires_at = @expires_at WHERE id = @id;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id = @id;

-- name: DeleteUserSessions :exec
DELETE FROM sessions WHERE user_id = @user_id;

-- name: CreatePasswordResetToken :exec
INSERT INTO password_reset_tokens (token_hash, user_id, expires_at)
VALUES (@token_hash, @user_id, @expires_at);

-- name: InvalidateUserResetTokens :exec
UPDATE password_reset_tokens SET used_at = now()
WHERE user_id = @user_id AND used_at IS NULL;

-- name: ConsumePasswordResetToken :one
UPDATE password_reset_tokens SET used_at = now()
WHERE token_hash = @token_hash AND used_at IS NULL AND expires_at > now()
RETURNING user_id;

-- name: LoginAttemptStats :one
SELECT count(*)::int AS attempts,
       coalesce(min(at), now())::timestamptz AS oldest
FROM login_attempts
WHERE key = @key AND at > @since;

-- name: RecordLoginAttempt :exec
INSERT INTO login_attempts (key) VALUES (@key);

-- name: ClearLoginAttempts :exec
DELETE FROM login_attempts WHERE key = @key;

-- name: PruneLoginAttempts :exec
DELETE FROM login_attempts WHERE at < @before;
