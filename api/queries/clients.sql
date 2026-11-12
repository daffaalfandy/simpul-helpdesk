-- name: UpsertClient :one
INSERT INTO clients (name) VALUES (@name)
ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
RETURNING id;

-- name: GetClientByName :one
SELECT id, name FROM clients WHERE name = @name;

-- name: ListClients :many
-- agent_id limits the list to clients assigned to that agent; NULL lists all.
-- Only requesters carry a client_id (users_requester_has_client), so this counts requesters.
SELECT c.id, c.name, c.contract_no, c.contract_start, c.contract_end, c.pic_name, c.pic_email,
       c.sla_plan_id, p.name AS sla_plan_name, c.created_at, c.updated_at,
       (SELECT count(*) FROM users u WHERE u.client_id = c.id)::int AS requester_count
FROM clients c
JOIN sla_plans p ON p.id = c.sla_plan_id
WHERE sqlc.narg(agent_id)::uuid IS NULL
   OR EXISTS (SELECT 1 FROM client_agents ca
              WHERE ca.client_id = c.id AND ca.user_id = sqlc.narg(agent_id)::uuid)
ORDER BY lower(c.name);

-- name: GetClient :one
SELECT c.id, c.name, c.contract_no, c.contract_start, c.contract_end, c.pic_name, c.pic_email,
       c.sla_plan_id, p.name AS sla_plan_name, c.created_at, c.updated_at,
       (SELECT count(*) FROM users u WHERE u.client_id = c.id)::int AS requester_count
FROM clients c
JOIN sla_plans p ON p.id = c.sla_plan_id
WHERE c.id = @id
  AND (sqlc.narg(agent_id)::uuid IS NULL
       OR EXISTS (SELECT 1 FROM client_agents ca
                  WHERE ca.client_id = c.id AND ca.user_id = sqlc.narg(agent_id)::uuid));

-- name: GetClientForUpdate :one
SELECT id, name, contract_no, contract_start, contract_end, pic_name, pic_email, sla_plan_id
FROM clients
WHERE id = @id
FOR UPDATE;

-- name: CreateClient :one
INSERT INTO clients (name, contract_no, contract_start, contract_end, pic_name, pic_email, sla_plan_id)
VALUES (@name, sqlc.narg(contract_no), sqlc.narg(contract_start), sqlc.narg(contract_end),
        sqlc.narg(pic_name), sqlc.narg(pic_email), @sla_plan_id)
RETURNING id;

-- name: UpdateClient :exec
UPDATE clients
SET name = @name,
    contract_no = sqlc.narg(contract_no),
    contract_start = sqlc.narg(contract_start),
    contract_end = sqlc.narg(contract_end),
    pic_name = sqlc.narg(pic_name),
    pic_email = sqlc.narg(pic_email),
    sla_plan_id = @sla_plan_id
WHERE id = @id;

-- name: UpsertClientDetails :one
-- Seed only: create or refresh a client by name.
INSERT INTO clients (name, contract_no, contract_start, contract_end, pic_name, pic_email, sla_plan_id)
VALUES (@name, sqlc.narg(contract_no), sqlc.narg(contract_start), sqlc.narg(contract_end),
        sqlc.narg(pic_name), sqlc.narg(pic_email), @sla_plan_id)
ON CONFLICT (name) DO UPDATE
SET contract_no = EXCLUDED.contract_no,
    contract_start = EXCLUDED.contract_start,
    contract_end = EXCLUDED.contract_end,
    pic_name = EXCLUDED.pic_name,
    pic_email = EXCLUDED.pic_email,
    sla_plan_id = EXCLUDED.sla_plan_id
RETURNING id;

-- name: DeleteClient :exec
DELETE FROM clients WHERE id = @id;

-- name: CountClientUsers :one
SELECT count(*)::int FROM users WHERE client_id = @client_id;

-- name: ListClientAgents :many
-- client_id NULL lists the agents of every client.
SELECT ca.client_id, u.id, u.name
FROM client_agents ca
JOIN users u ON u.id = ca.user_id
WHERE sqlc.narg(client_id)::uuid IS NULL OR ca.client_id = sqlc.narg(client_id)::uuid
ORDER BY u.name, u.id;

-- name: ListClientAgentIDs :many
SELECT user_id FROM client_agents WHERE client_id = @client_id ORDER BY user_id;

-- name: DeleteClientAgents :exec
DELETE FROM client_agents WHERE client_id = @client_id;

-- name: InsertClientAgents :exec
INSERT INTO client_agents (client_id, user_id)
SELECT @client_id, unnest(@user_ids::uuid[])
ON CONFLICT DO NOTHING;

-- name: CountUsersWithRoleAndStatus :one
SELECT count(*)::int FROM users
WHERE id = ANY(@ids::uuid[]) AND role = @role AND status = @status;

-- name: ListAgentClientIDs :many
SELECT client_id FROM client_agents WHERE user_id = @user_id ORDER BY client_id;
