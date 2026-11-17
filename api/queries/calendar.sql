-- name: GetOrganizationTimezone :one
SELECT timezone FROM organizations WHERE id = @organization_id;

-- name: LockOrganization :exec
-- Serialises changes to one organization's settings.
SELECT 1 FROM organizations WHERE id = @organization_id FOR UPDATE;

-- name: LockInstall :exec
-- Serialises install-wide rules: the last administrator, first-run setup, new organizations.
SELECT 1 FROM install WHERE id FOR UPDATE;

-- name: ListBusinessHours :many
SELECT weekday, open_minute, close_minute FROM business_hours
WHERE organization_id = @organization_id
ORDER BY weekday;

-- name: DeleteBusinessHours :exec
DELETE FROM business_hours WHERE organization_id = @organization_id;

-- name: InsertBusinessHours :exec
INSERT INTO business_hours (organization_id, weekday, open_minute, close_minute)
VALUES (@organization_id, @weekday, @open_minute, @close_minute);

-- name: ListHolidays :many
-- from/to bound the dates (inclusive); NULL is open-ended.
SELECT id, date, name
FROM holidays
WHERE organization_id = @organization_id
  AND (sqlc.narg(from_date)::date IS NULL OR date >= sqlc.narg(from_date)::date)
  AND (sqlc.narg(to_date)::date IS NULL OR date <= sqlc.narg(to_date)::date)
ORDER BY date;

-- name: CreateHoliday :one
INSERT INTO holidays (organization_id, date, name) VALUES (@organization_id, @date, @name) RETURNING id;

-- name: InsertHolidayIfMissing :execrows
INSERT INTO holidays (organization_id, date, name) VALUES (@organization_id, @date, @name)
ON CONFLICT (organization_id, date) DO NOTHING;

-- name: DeleteHoliday :one
DELETE FROM holidays WHERE id = @id AND organization_id = @organization_id RETURNING date, name;
