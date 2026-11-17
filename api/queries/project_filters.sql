-- A user's saved project filter in one organization (owner, 17 Nov). The board, its counts and
-- the staff ticket list show only these projects; no rows means every project. agent_id limits
-- it to the projects of an agent's clients, as the project list does.

-- name: ListProjectFilter :many
-- The saved projects the user can still see.
SELECT f.project_id
FROM user_project_filters f
JOIN projects p ON p.id = f.project_id AND p.organization_id = f.organization_id
WHERE f.user_id = @user_id AND f.organization_id = @organization_id
  AND (sqlc.narg(agent_id)::uuid IS NULL OR EXISTS (
        SELECT 1 FROM client_agents ca WHERE ca.client_id = p.client_id AND ca.user_id = sqlc.narg(agent_id)::uuid))
ORDER BY f.project_id;

-- name: CountVisibleProjects :one
-- How many of @ids are projects of the organization the user can see.
SELECT count(*)::int
FROM projects p
WHERE p.organization_id = @organization_id AND p.id = ANY(@ids::uuid[])
  AND (sqlc.narg(agent_id)::uuid IS NULL OR EXISTS (
        SELECT 1 FROM client_agents ca WHERE ca.client_id = p.client_id AND ca.user_id = sqlc.narg(agent_id)::uuid));

-- name: ClearProjectFilter :exec
DELETE FROM user_project_filters WHERE user_id = @user_id AND organization_id = @organization_id;

-- name: AddToProjectFilter :exec
INSERT INTO user_project_filters (user_id, organization_id, project_id)
SELECT @user_id::uuid, @organization_id::uuid, unnest(@project_ids::uuid[]);
