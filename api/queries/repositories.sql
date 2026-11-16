-- name: InsertRepository :exec
-- The id comes from the caller: the sealed secret is bound to it.
INSERT INTO repositories (id, provider, host, full_name, web_url, webhook_secret, created_by)
VALUES (@id, @provider, @host, @full_name, @web_url, @webhook_secret, @created_by);

-- name: GetRepositoryByName :one
SELECT id, hidden FROM repositories
WHERE provider = @provider AND lower(host) = lower(@host) AND lower(full_name) = lower(@full_name);

-- name: RestoreRepository :exec
-- A removed repository added again: shown again with a new secret and no old error.
UPDATE repositories
SET hidden = false, host = @host, full_name = @full_name, web_url = @web_url, webhook_secret = @webhook_secret,
    last_error = NULL, last_error_at = NULL
WHERE id = @id;

-- name: GetRepository :one
SELECT id, provider, host, full_name, web_url, external_id, webhook_secret, hidden,
       last_delivery_at, last_error, last_error_at, created_at
FROM repositories WHERE id = @id;

-- name: GetRepositoryForUpdate :one
SELECT id, provider, host, full_name, web_url, hidden FROM repositories WHERE id = @id FOR UPDATE;

-- name: ListRepositories :many
SELECT id, provider, host, full_name, web_url, external_id, last_delivery_at, last_error, last_error_at, created_at
FROM repositories
WHERE NOT hidden
ORDER BY provider, lower(full_name);

-- name: ListLinkedProjects :many
-- Projects that read code from a repository, for the Integrations page.
SELECT p.id, p.name, p.repository_id, p.client_id, c.name AS client_name
FROM projects p
JOIN clients c ON c.id = p.client_id
WHERE p.repository_id IS NOT NULL
ORDER BY lower(c.name), lower(p.name);

-- name: LockProjectsForLink :many
-- The projects a link change touches: the ones asked for and the ones linked now.
SELECT p.id, p.name, p.repository_id, p.dod_config
FROM projects p
WHERE p.id = ANY(@ids::uuid[]) OR p.repository_id = @repository_id
ORDER BY p.id
FOR UPDATE;

-- name: SetProjectsRepository :exec
UPDATE projects SET repository_id = sqlc.narg(repository_id) WHERE id = ANY(@ids::uuid[]);

-- name: SetRepositorySecret :exec
UPDATE repositories SET webhook_secret = @webhook_secret, last_error = NULL, last_error_at = NULL WHERE id = @id;

-- name: HideRepository :exec
UPDATE repositories SET hidden = true WHERE id = @id;

-- name: DeleteRepository :exec
DELETE FROM repositories WHERE id = @id;

-- name: RepositoryHasEvents :one
SELECT EXISTS (SELECT 1 FROM code_events WHERE repository_id = @id);
