-- name: GetOrganizationTimezone :one
SELECT timezone FROM organization WHERE id;

-- name: LockOrganization :exec
-- Serialises changes to organisation-wide settings.
SELECT 1 FROM organization WHERE id FOR UPDATE;

-- name: ListBusinessHours :many
SELECT weekday, open_minute, close_minute FROM business_hours ORDER BY weekday;

-- name: DeleteBusinessHours :exec
DELETE FROM business_hours;

-- name: InsertBusinessHours :exec
INSERT INTO business_hours (weekday, open_minute, close_minute)
VALUES (@weekday, @open_minute, @close_minute);

-- name: ListHolidays :many
-- from/to bound the dates (inclusive); NULL is open-ended.
SELECT id, date, name
FROM holidays
WHERE (sqlc.narg(from_date)::date IS NULL OR date >= sqlc.narg(from_date)::date)
  AND (sqlc.narg(to_date)::date IS NULL OR date <= sqlc.narg(to_date)::date)
ORDER BY date;

-- name: CreateHoliday :one
INSERT INTO holidays (date, name) VALUES (@date, @name) RETURNING id;

-- name: InsertHolidayIfMissing :execrows
INSERT INTO holidays (date, name) VALUES (@date, @name)
ON CONFLICT (date) DO NOTHING;

-- name: DeleteHoliday :one
DELETE FROM holidays WHERE id = @id RETURNING date, name;
