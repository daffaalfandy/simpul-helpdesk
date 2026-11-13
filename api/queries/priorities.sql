-- name: ListPriorityLevels :many
SELECT code, name, description FROM priorities ORDER BY code;

-- name: ListPriorityLevelsForUpdate :many
SELECT code, name, description FROM priorities ORDER BY code FOR UPDATE;

-- name: UpdatePriorityLevel :exec
UPDATE priorities SET name = @name, description = @description WHERE code = @code;
