-- name: InsertRepository :exec
-- The id comes from the caller: the sealed secret is bound to it.
INSERT INTO repositories (id, organization_id, provider, host, full_name, web_url, webhook_secret, created_by)
VALUES (@id, @organization_id, @provider, @host, @full_name, @web_url, @webhook_secret, @created_by);

-- name: GetRepositoryByName :one
SELECT id, hidden FROM repositories
WHERE organization_id = @organization_id AND provider = @provider AND lower(host) = lower(@host) AND lower(full_name) = lower(@full_name);

-- name: RestoreRepository :exec
-- A removed repository added again: shown again with a new secret and no old error.
UPDATE repositories
SET hidden = false, host = @host, full_name = @full_name, web_url = @web_url, webhook_secret = @webhook_secret,
    last_error = NULL, last_error_at = NULL
WHERE id = @id;

-- name: GetRepository :one
-- By id alone: webhook intake and jobs, which then work in the repository's organization.
SELECT id, organization_id, provider, host, full_name, web_url, external_id, webhook_secret, hidden,
       last_delivery_at, last_error, last_error_at, created_at
FROM repositories WHERE id = @id;

-- name: GetRepositoryForUpdate :one
SELECT id, provider, host, full_name, web_url, hidden FROM repositories
WHERE id = @id AND organization_id = @organization_id FOR UPDATE;

-- name: ListRepositories :many
SELECT id, provider, host, full_name, web_url, external_id, last_delivery_at, last_error, last_error_at, created_at
FROM repositories
WHERE organization_id = @organization_id AND NOT hidden
ORDER BY provider, lower(full_name);

-- name: ListLinkedProjects :many
-- Projects that read code from a repository, for the Integrations page.
SELECT p.id, p.name, p.repository_id, p.client_id, c.name AS client_name
FROM projects p
JOIN clients c ON c.id = p.client_id
WHERE p.organization_id = @organization_id AND p.repository_id IS NOT NULL
ORDER BY lower(c.name), lower(p.name);

-- name: LockProjectsForLink :many
-- The projects a link change touches: the ones asked for and the ones linked now.
SELECT p.id, p.name, p.repository_id
FROM projects p
WHERE p.organization_id = @organization_id
  AND (p.id = ANY(@ids::uuid[]) OR p.repository_id = @repository_id)
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

-- name: MarkSeedRepositoryDelivered :exec
-- Seed only: the demo repositories have sent webhooks.
UPDATE repositories SET last_delivery_at = @at WHERE id = @id;
