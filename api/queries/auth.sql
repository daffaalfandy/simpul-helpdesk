-- name: CreateSession :one
INSERT INTO sessions (token_hash, user_id, expires_at, ip, user_agent)
VALUES (@token_hash, @user_id, @expires_at, sqlc.narg(ip), sqlc.narg(user_agent))
RETURNING id;

-- name: GetSessionPrincipal :one
SELECT s.id AS session_id, s.created_at AS session_created_at, s.last_seen_at, s.expires_at,
       u.id AS user_id, u.email, u.name, u.role, u.client_id, c.name AS client_name,
       c.organization_id AS client_organization_id, u.job_title, u.status, u.password_changed_at,
       u.last_organization_id
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

-- name: GetPasswordResetTokenUser :one
-- A live reset link of an active account, read before any password is hashed.
SELECT t.user_id
FROM password_reset_tokens t
JOIN users u ON u.id = t.user_id
WHERE t.token_hash = @token_hash AND t.used_at IS NULL AND t.expires_at > now() AND u.status = @active;

-- name: ConsumePasswordResetToken :one
UPDATE password_reset_tokens t SET used_at = now()
FROM users u
WHERE t.token_hash = @token_hash AND t.used_at IS NULL AND t.expires_at > now()
  AND u.id = t.user_id AND u.status = @active
RETURNING t.user_id;

-- name: LoginAttemptStats :one
SELECT count(*)::int AS attempts,
       coalesce(min(at), now())::timestamptz AS oldest
FROM login_attempts
WHERE key = @key AND at > @since;

-- name: RecordLoginAttempt :exec
INSERT INTO login_attempts (key) VALUES (@key);

-- name: ReserveLoginAttempt :one
INSERT INTO login_attempts (key) VALUES (@key) RETURNING id;

-- name: DeleteLoginAttempt :exec
DELETE FROM login_attempts WHERE id = @id;

-- name: ClearLoginAttempts :exec
DELETE FROM login_attempts WHERE key = @key;

-- name: PruneLoginAttempts :exec
DELETE FROM login_attempts WHERE at < @before;

-- name: PurgeExpiredSessions :exec
DELETE FROM sessions WHERE expires_at < @before;

-- name: PurgeSpentResetTokens :exec
DELETE FROM password_reset_tokens WHERE expires_at < @before OR used_at < @before;

-- name: CreateUserInvite :exec
INSERT INTO user_invites (token_hash, user_id, invited_by, expires_at)
VALUES (@token_hash, @user_id, sqlc.narg(invited_by), @expires_at);

-- name: InvalidateUserInvites :exec
UPDATE user_invites SET used_at = now() WHERE user_id = @user_id AND used_at IS NULL;

-- name: GetUserInviteUser :one
-- A live invite of an account still waiting for its first password, read before any password
-- is hashed.
SELECT i.user_id
FROM user_invites i
JOIN users u ON u.id = i.user_id
WHERE i.token_hash = @token_hash AND i.used_at IS NULL AND i.expires_at > now() AND u.status = @invited;

-- name: ConsumeUserInvite :one
-- Single use, unexpired, and only while the account still waits for its first password.
UPDATE user_invites i SET used_at = now()
FROM users u
WHERE i.token_hash = @token_hash AND i.used_at IS NULL AND i.expires_at > now()
  AND u.id = i.user_id AND u.status = @invited
RETURNING i.user_id;
