-- name: ListSlaPlans :many
SELECT p.id, p.name, p.is_default,
       (SELECT count(*) FROM clients c WHERE c.sla_plan_id = p.id)::int AS client_count
FROM sla_plans p
ORDER BY p.is_default DESC, lower(p.name);

-- name: GetSlaPlan :one
SELECT p.id, p.name, p.is_default,
       (SELECT count(*) FROM clients c WHERE c.sla_plan_id = p.id)::int AS client_count
FROM sla_plans p
WHERE p.id = @id;

-- name: GetSlaPlanForUpdate :one
SELECT id, name, is_default FROM sla_plans WHERE id = @id FOR UPDATE;

-- name: GetDefaultSlaPlanForUpdate :one
SELECT id, name FROM sla_plans WHERE is_default FOR UPDATE;

-- name: SlaPlanExists :one
SELECT EXISTS (SELECT 1 FROM sla_plans WHERE id = @id);

-- name: CreateSlaPlan :one
INSERT INTO sla_plans (name) VALUES (@name) RETURNING id;

-- name: UpsertSlaPlanByName :one
-- Seed only.
INSERT INTO sla_plans (name) VALUES (@name)
ON CONFLICT (lower(name)) DO UPDATE SET name = EXCLUDED.name
RETURNING id;

-- name: RenameSlaPlan :exec
UPDATE sla_plans SET name = @name WHERE id = @id;

-- name: SetSlaPlanDefault :exec
UPDATE sla_plans SET is_default = @is_default WHERE id = @id;

-- name: DeleteSlaPlan :exec
DELETE FROM sla_plans WHERE id = @id;

-- name: CountClientsOnSlaPlan :one
SELECT count(*)::int FROM clients WHERE sla_plan_id = @plan_id;

-- name: ListSlaTargets :many
-- plan_id NULL lists the targets of every plan.
SELECT plan_id, priority, first_response_minutes, resolve_minutes, deliver_minutes,
       deliver_from_catalog, calendar
FROM sla_targets
WHERE sqlc.narg(plan_id)::uuid IS NULL OR plan_id = sqlc.narg(plan_id)::uuid
ORDER BY plan_id, priority;

-- name: UpsertSlaTarget :exec
INSERT INTO sla_targets (plan_id, priority, first_response_minutes, resolve_minutes, deliver_minutes,
                         deliver_from_catalog, calendar)
VALUES (@plan_id, @priority, @first_response_minutes, @resolve_minutes, @deliver_minutes,
        @deliver_from_catalog, @calendar)
ON CONFLICT (plan_id, priority) DO UPDATE
SET first_response_minutes = EXCLUDED.first_response_minutes,
    resolve_minutes = EXCLUDED.resolve_minutes,
    deliver_minutes = EXCLUDED.deliver_minutes,
    deliver_from_catalog = EXCLUDED.deliver_from_catalog,
    calendar = EXCLUDED.calendar;
