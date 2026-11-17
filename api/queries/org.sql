-- name: GetOrganization :one
SELECT id, name, ticket_prefix, timezone, footer_note, logo_key, logo_content_type, logo_updated_at, created_at
FROM organizations
WHERE id = @id;

-- name: GetFirstOrganization :one
-- The organization made first: the one a single-organization install shows on its sign-in page,
-- and the owner of logo links in emails sent before organizations existed.
SELECT id, name, ticket_prefix, timezone, footer_note, logo_key, logo_content_type, logo_updated_at, created_at
FROM organizations
ORDER BY created_at, id
LIMIT 1;

-- name: CountOrganizations :one
SELECT count(*)::int FROM organizations;

-- name: CreateOrganization :one
INSERT INTO organizations (name, ticket_prefix, timezone)
VALUES (@name, @ticket_prefix, @timezone)
RETURNING id;

-- name: InsertDefaultPriorities :exec
-- Every organization starts with the four priorities and these names (owner, 13 Nov).
INSERT INTO priorities (organization_id, code, name, description) VALUES
  (@organization_id, 'P1', 'Critical', 'A public service or the whole office is down'),
  (@organization_id, 'P2', 'High', 'A team or department cannot work'),
  (@organization_id, 'P3', 'Standard', 'One person is slowed down, with a workaround'),
  (@organization_id, 'P4', 'Low', 'Minor issue or question, no rush');

-- name: InsertDefaultBusinessHours :exec
-- Monday to Friday, 08:00–17:00.
INSERT INTO business_hours (organization_id, weekday, open_minute, close_minute)
SELECT @organization_id, d, 480, 1020 FROM generate_series(1, 5) AS d;

-- name: UpdateOrganizationProfile :exec
-- Seed only: the demo organisation's name and footer.
UPDATE organizations
SET name = @name, footer_note = sqlc.narg(footer_note)
WHERE id = @id;

-- name: UpdateOrganizationSettings :exec
UPDATE organizations
SET name = @name, ticket_prefix = @ticket_prefix, timezone = @timezone
WHERE id = @id;

-- name: SetOrganizationLogo :exec
-- A NULL key removes the logo.
UPDATE organizations
SET logo_key = sqlc.narg(logo_key), logo_content_type = sqlc.narg(logo_content_type),
    logo_updated_at = sqlc.narg(logo_updated_at)
WHERE id = @id;

-- name: GetInstall :one
SELECT setup_completed_at FROM install WHERE id;

-- name: CompleteSetup :exec
-- The first admin exists: first-run setup closes for good.
UPDATE install SET setup_completed_at = coalesce(setup_completed_at, now()) WHERE id;

-- name: ListOrganizationIDs :many
-- For jobs that run organization by organization.
SELECT id FROM organizations ORDER BY created_at, id;

-- name: GetOrganizationByPrefix :one
-- The CLI names an organization by its ticket prefix, which is unique.
SELECT id FROM organizations WHERE ticket_prefix = upper(@ticket_prefix);
