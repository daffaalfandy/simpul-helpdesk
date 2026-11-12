-- name: GetUserByEmail :one
SELECT u.id, u.email, u.name, u.password_hash, u.role, u.client_id, c.name AS client_name,
       u.job_title, u.status
FROM users u
LEFT JOIN clients c ON c.id = u.client_id
WHERE u.email = @email;

-- name: GetUserByID :one
SELECT u.id, u.email, u.name, u.role, u.client_id, c.name AS client_name, u.job_title, u.status
FROM users u
LEFT JOIN clients c ON c.id = u.client_id
WHERE u.id = @id;

-- name: ListUsers :many
SELECT u.id, u.email, u.name, u.role, u.client_id, c.name AS client_name, u.job_title, u.status,
       u.last_login_at
FROM users u
LEFT JOIN clients c ON c.id = u.client_id
ORDER BY u.name, u.email;

-- name: CreateUser :one
INSERT INTO users (email, name, password_hash, role, client_id, job_title)
VALUES (@email, @name, @password_hash, @role, sqlc.narg(client_id), sqlc.narg(job_title))
RETURNING id;

-- name: UpsertUserByEmail :one
INSERT INTO users (email, name, password_hash, role, client_id, job_title, status)
VALUES (@email, @name, @password_hash, @role, sqlc.narg(client_id), sqlc.narg(job_title), @status)
ON CONFLICT (email) DO UPDATE
SET name = EXCLUDED.name,
    password_hash = EXCLUDED.password_hash,
    role = EXCLUDED.role,
    client_id = EXCLUDED.client_id,
    job_title = EXCLUDED.job_title,
    status = EXCLUDED.status
RETURNING id;

-- name: SetUserPassword :exec
UPDATE users SET password_hash = @password_hash, password_changed_at = now() WHERE id = @id;

-- name: UpdateUserPasswordHash :exec
-- Rehash with new parameters; the password itself is unchanged, so sessions stay valid.
UPDATE users SET password_hash = @password_hash WHERE id = @id;

-- name: SetUserStatus :exec
UPDATE users SET status = @status WHERE id = @id;

-- name: TouchUserLogin :exec
UPDATE users SET last_login_at = now() WHERE id = @id;
