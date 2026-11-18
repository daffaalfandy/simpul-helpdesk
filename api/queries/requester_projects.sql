-- Which projects of their client each requester reports on (10b, owner 18 Nov). A requester sees
-- the tickets of those projects and the tickets they reported themselves; internal/tickets
-- access.go turns that into the scope_requester_id condition of the ticket queries.

-- name: LockClientRequester :one
-- Serialises changes to one requester's projects.
SELECT id, name, email, job_title, status FROM users
WHERE id = @id AND client_id = @client_id AND role = @role
FOR NO KEY UPDATE;

-- name: ListRequesterProjectIDs :many
SELECT project_id FROM requester_projects
WHERE user_id = @user_id AND organization_id = @organization_id
ORDER BY project_id;

-- name: ListClientRequesterProjects :many
-- Every requester's projects at one client, for Settings › Clients.
SELECT user_id, project_id FROM requester_projects
WHERE client_id = @client_id AND organization_id = @organization_id
ORDER BY user_id, project_id;

-- name: CountClientProjectsIn :one
-- How many of ids are projects of the client: a caller with distinct ids compares it with their
-- number.
SELECT count(*)::int FROM projects
WHERE id = ANY(@ids::uuid[]) AND client_id = @client_id AND organization_id = @organization_id;

-- name: AddRequesterProjects :many
-- Returns the projects newly linked.
INSERT INTO requester_projects (user_id, project_id, client_id, organization_id)
SELECT @user_id, unnest(@project_ids::uuid[]), @client_id, @organization_id
ON CONFLICT DO NOTHING
RETURNING project_id;

-- name: RemoveRequesterProjects :many
-- Returns the projects unlinked.
DELETE FROM requester_projects
WHERE user_id = @user_id AND organization_id = @organization_id AND project_id = ANY(@project_ids::uuid[])
RETURNING project_id;

-- name: LinkRequesterToClientProjects :exec
-- A requester made from the command line reports on every project of their client.
INSERT INTO requester_projects (user_id, project_id, client_id, organization_id)
SELECT @user_id, p.id, p.client_id, p.organization_id
FROM projects p
WHERE p.client_id = @client_id AND p.organization_id = @organization_id
ON CONFLICT DO NOTHING;

-- name: LinkClientRequestersToProject :many
-- A client's first project: every requester of the client reports on it. Returns who was linked.
INSERT INTO requester_projects (user_id, project_id, client_id, organization_id)
SELECT u.id, @project_id, u.client_id, @organization_id
FROM users u
WHERE u.client_id = @client_id AND u.role = @role
ORDER BY u.id
ON CONFLICT DO NOTHING
RETURNING user_id;

-- name: IsRequesterOnProject :one
SELECT EXISTS (SELECT 1 FROM requester_projects
               WHERE user_id = @user_id AND project_id = @project_id AND organization_id = @organization_id);
