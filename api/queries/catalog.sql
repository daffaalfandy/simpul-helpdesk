-- name: ListCatalogItems :many
-- active NULL lists every item.
SELECT i.id, i.name, i.description, i.sort_order, i.active,
       (SELECT count(*) FROM tickets t WHERE t.catalog_item_id = i.id)::int AS ticket_count
FROM catalog_items i
WHERE i.organization_id = @organization_id
  AND (sqlc.narg(active)::boolean IS NULL OR i.active = sqlc.narg(active)::boolean)
ORDER BY i.sort_order, lower(i.name);

-- name: GetCatalogItem :one
SELECT i.id, i.name, i.description, i.sort_order, i.active,
       (SELECT count(*) FROM tickets t WHERE t.catalog_item_id = i.id)::int AS ticket_count
FROM catalog_items i
WHERE i.id = @id AND i.organization_id = @organization_id;

-- name: GetCatalogItemForUpdate :one
SELECT id, name, description, sort_order, active FROM catalog_items
WHERE id = @id AND organization_id = @organization_id FOR UPDATE;

-- name: GetActiveCatalogItem :one
SELECT id, name FROM catalog_items WHERE id = @id AND organization_id = @organization_id AND active;

-- name: CreateCatalogItem :one
INSERT INTO catalog_items (organization_id, name, description, sort_order, active)
VALUES (@organization_id, @name, sqlc.narg(description), @sort_order, @active)
RETURNING id;

-- name: UpdateCatalogItem :exec
UPDATE catalog_items
SET name = @name, description = sqlc.narg(description), sort_order = @sort_order, active = @active
WHERE id = @id AND organization_id = @organization_id;

-- name: DeleteCatalogItem :exec
DELETE FROM catalog_items WHERE id = @id AND organization_id = @organization_id;

-- name: CountCatalogItemTickets :one
SELECT count(*)::int FROM tickets WHERE catalog_item_id = @catalog_item_id;

-- name: UpsertCatalogItemByName :one
-- Seed only.
INSERT INTO catalog_items (organization_id, name, description, sort_order)
VALUES (@organization_id, @name, sqlc.narg(description), @sort_order)
ON CONFLICT (organization_id, lower(name)) DO UPDATE
SET description = EXCLUDED.description, sort_order = EXCLUDED.sort_order
RETURNING id;
