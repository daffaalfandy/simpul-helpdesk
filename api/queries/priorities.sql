-- name: ListPriorityLevels :many
SELECT code, name, description FROM priorities WHERE organization_id = @organization_id ORDER BY code;

-- name: ListPriorityLevelsForUpdate :many
SELECT code, name, description FROM priorities WHERE organization_id = @organization_id ORDER BY code FOR UPDATE;

-- name: UpdatePriorityLevel :exec
UPDATE priorities SET name = @name, description = @description
WHERE organization_id = @organization_id AND code = @code;
