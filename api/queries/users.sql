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

-- name: CreateInvitedUser :one
-- No password: the user can't sign in until an invite (M9) sets one.
INSERT INTO users (email, name, role, client_id, job_title, status)
VALUES (@email, @name, @role, sqlc.narg(client_id), sqlc.narg(job_title), @status)
RETURNING id;

-- name: ListClientRequesters :many
SELECT id, name, email, job_title, status
FROM users
WHERE client_id = @client_id AND role = @role AND status = ANY(@statuses::text[])
ORDER BY lower(name), email;

-- name: GetClientRequester :one
SELECT id, name, status FROM users WHERE id = @id AND client_id = @client_id AND role = @role;

-- name: ListAssignableUsers :many
-- Who can take a client's tickets: active users whose role sees every client, plus active
-- agents assigned to the client. open_statuses counts each person's open workload.
SELECT u.id, u.name, u.role, u.job_title,
       (SELECT count(*) FROM tickets t
        WHERE t.assignee_id = u.id AND t.status = ANY(@open_statuses::text[]))::int AS open_ticket_count
FROM users u
WHERE u.status = @status
  AND (u.role = ANY(@all_client_roles::text[])
       OR (u.role = @agent_role
           AND EXISTS (SELECT 1 FROM client_agents ca WHERE ca.client_id = @client_id AND ca.user_id = u.id)))
ORDER BY lower(u.name), u.id;

-- name: GetAssignableUser :one
-- Same rule as ListAssignableUsers, for one user.
SELECT u.id, u.name
FROM users u
WHERE u.id = @id
  AND u.status = @status
  AND (u.role = ANY(@all_client_roles::text[])
       OR (u.role = @agent_role
           AND EXISTS (SELECT 1 FROM client_agents ca WHERE ca.client_id = @client_id AND ca.user_id = u.id)));

-- name: ListTeamMembers :many
-- Everyone but client staff.
SELECT u.id, u.email, u.name, u.role, u.job_title, u.status, u.last_login_at
FROM users u
WHERE u.role <> @requester_role
ORDER BY lower(u.name), u.email;

-- name: ListLatestInvites :many
-- When each person's latest invite went out.
SELECT user_id, max(created_at)::timestamptz AS sent_at FROM user_invites GROUP BY user_id;

-- name: CountUsersByRole :many
-- People with access or an invite: suspended accounts aren't counted.
SELECT role, count(*)::int AS n FROM users WHERE status <> @suspended GROUP BY role;

-- name: GetUserForUpdate :one
SELECT id, email, name, role, status, client_id, (password_hash IS NOT NULL)::boolean AS has_password
FROM users WHERE id = @id FOR UPDATE;

-- name: CountActiveAdmins :one
SELECT count(*)::int FROM users WHERE role = @admin_role AND status = @active;

-- name: UpdateUserRole :exec
UPDATE users SET role = @role WHERE id = @id;

-- name: DeleteClientAgentsForUser :exec
-- An agent who leaves (suspended, or another role) stops being linked to clients; the tickets
-- assigned to them stay theirs.
DELETE FROM client_agents WHERE user_id = @user_id;
