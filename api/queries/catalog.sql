-- name: ListSystems :many
-- active NULL lists every system.
SELECT s.id, s.name, s.description, s.sort_order, s.active,
       (SELECT count(*) FROM tickets t WHERE t.system_id = s.id)::int AS ticket_count
FROM systems s
WHERE sqlc.narg(active)::boolean IS NULL OR s.active = sqlc.narg(active)::boolean
ORDER BY s.sort_order, lower(s.name);

-- name: GetSystem :one
SELECT s.id, s.name, s.description, s.sort_order, s.active,
       (SELECT count(*) FROM tickets t WHERE t.system_id = s.id)::int AS ticket_count
FROM systems s
WHERE s.id = @id;

-- name: GetSystemForUpdate :one
SELECT id, name, description, sort_order, active FROM systems WHERE id = @id FOR UPDATE;

-- name: GetActiveSystem :one
SELECT id, name FROM systems WHERE id = @id AND active;

-- name: CreateSystem :one
INSERT INTO systems (name, description, sort_order, active)
VALUES (@name, sqlc.narg(description), @sort_order, @active)
RETURNING id;

-- name: UpdateSystem :exec
UPDATE systems
SET name = @name, description = sqlc.narg(description), sort_order = @sort_order, active = @active
WHERE id = @id;

-- name: DeleteSystem :exec
DELETE FROM systems WHERE id = @id;

-- name: CountSystemTickets :one
SELECT count(*)::int FROM tickets WHERE system_id = @system_id;

-- name: UpsertSystemByName :one
-- Seed only.
INSERT INTO systems (name, description, sort_order)
VALUES (@name, sqlc.narg(description), @sort_order)
ON CONFLICT (lower(name)) DO UPDATE SET description = EXCLUDED.description, sort_order = EXCLUDED.sort_order
RETURNING id;

-- name: ListCatalogItems :many
-- active NULL lists every item.
SELECT i.id, i.name, i.description, i.delivery_minutes, i.sort_order, i.active,
       (SELECT count(*) FROM tickets t WHERE t.catalog_item_id = i.id)::int AS ticket_count
FROM catalog_items i
WHERE sqlc.narg(active)::boolean IS NULL OR i.active = sqlc.narg(active)::boolean
ORDER BY i.sort_order, lower(i.name);

-- name: GetCatalogItem :one
SELECT i.id, i.name, i.description, i.delivery_minutes, i.sort_order, i.active,
       (SELECT count(*) FROM tickets t WHERE t.catalog_item_id = i.id)::int AS ticket_count
FROM catalog_items i
WHERE i.id = @id;

-- name: GetCatalogItemForUpdate :one
SELECT id, name, description, delivery_minutes, sort_order, active FROM catalog_items WHERE id = @id FOR UPDATE;

-- name: GetActiveCatalogItem :one
SELECT id, name, delivery_minutes FROM catalog_items WHERE id = @id AND active;

-- name: CreateCatalogItem :one
INSERT INTO catalog_items (name, description, delivery_minutes, sort_order, active)
VALUES (@name, sqlc.narg(description), @delivery_minutes, @sort_order, @active)
RETURNING id;

-- name: UpdateCatalogItem :exec
UPDATE catalog_items
SET name = @name, description = sqlc.narg(description), delivery_minutes = @delivery_minutes,
    sort_order = @sort_order, active = @active
WHERE id = @id;

-- name: DeleteCatalogItem :exec
DELETE FROM catalog_items WHERE id = @id;

-- name: CountCatalogItemTickets :one
SELECT count(*)::int FROM tickets WHERE catalog_item_id = @catalog_item_id;

-- name: UpsertCatalogItemByName :one
-- Seed only.
INSERT INTO catalog_items (name, description, delivery_minutes, sort_order)
VALUES (@name, sqlc.narg(description), @delivery_minutes, @sort_order)
ON CONFLICT (lower(name)) DO UPDATE
SET description = EXCLUDED.description, delivery_minutes = EXCLUDED.delivery_minutes,
    sort_order = EXCLUDED.sort_order
RETURNING id;
