-- name: UpsertClient :one
INSERT INTO clients (name) VALUES (@name)
ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
RETURNING id;

-- name: GetClientByName :one
SELECT id, name FROM clients WHERE name = @name;
