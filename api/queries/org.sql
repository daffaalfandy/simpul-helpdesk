-- name: GetOrganization :one
SELECT name, ticket_prefix, timezone, footer_note, logo_key, logo_content_type, logo_updated_at, setup_completed_at
FROM organization
WHERE id;

-- name: UpdateOrganizationProfile :exec
-- Seed only: the demo organisation's name and footer.
UPDATE organization
SET name = @name, footer_note = sqlc.narg(footer_note)
WHERE id;

-- name: UpdateOrganizationSettings :exec
UPDATE organization
SET name = @name, ticket_prefix = @ticket_prefix, timezone = @timezone
WHERE id;

-- name: SetOrganizationLogo :exec
-- A NULL key removes the logo.
UPDATE organization
SET logo_key = sqlc.narg(logo_key), logo_content_type = sqlc.narg(logo_content_type),
    logo_updated_at = sqlc.narg(logo_updated_at)
WHERE id;

-- name: CompleteSetup :exec
-- The first admin exists: first-run setup closes for good.
UPDATE organization SET setup_completed_at = coalesce(setup_completed_at, now()) WHERE id;
