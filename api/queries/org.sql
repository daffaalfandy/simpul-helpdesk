-- name: GetOrganization :one
SELECT name, logo_url, ticket_prefix, timezone, footer_note
FROM organization
WHERE id;

-- name: UpdateOrganizationProfile :exec
UPDATE organization
SET name = @name, footer_note = sqlc.narg(footer_note)
WHERE id;
