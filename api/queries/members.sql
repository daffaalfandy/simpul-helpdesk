-- name: ListUserMemberships :many
-- The organizations a staff member works in, with their role in each.
SELECT m.organization_id, m.role, m.created_at, o.name, o.logo_key, o.logo_content_type, o.logo_updated_at
FROM organization_members m
JOIN organizations o ON o.id = m.organization_id
WHERE m.user_id = @user_id
ORDER BY lower(o.name), o.id;

-- name: ListOrganizations :many
-- Every organization, for administrators.
SELECT id, name, logo_key, logo_content_type, logo_updated_at
FROM organizations
ORDER BY lower(name), id;

-- name: UpsertMembership :exec
INSERT INTO organization_members (user_id, organization_id, role)
VALUES (@user_id, @organization_id, @role)
ON CONFLICT (user_id, organization_id) DO UPDATE SET role = EXCLUDED.role;

-- name: GetMembershipForUpdate :one
SELECT role FROM organization_members
WHERE user_id = @user_id AND organization_id = @organization_id
FOR UPDATE;

-- name: DeleteMembership :exec
DELETE FROM organization_members WHERE user_id = @user_id AND organization_id = @organization_id;

-- name: DeleteMemberships :exec
-- A staff member who becomes an administrator works in every organization.
DELETE FROM organization_members WHERE user_id = @user_id;

-- name: SetLastOrganization :exec
UPDATE users SET last_organization_id = @organization_id WHERE id = @id;

-- name: CountActiveAgentMembers :one
-- How many of ids are active agents of the organization (the ones a client can be linked to).
SELECT count(*)::int
FROM users u
JOIN organization_members m ON m.user_id = u.id AND m.organization_id = @organization_id
WHERE u.id = ANY(@ids::uuid[]) AND m.role = @agent_role AND u.status = @status;

-- name: ListActiveSupervisorIDs :many
-- The organization's active supervisors (breach and Definition of Done alerts).
SELECT u.id
FROM users u
JOIN organization_members m ON m.user_id = u.id AND m.organization_id = @organization_id
WHERE m.role = @supervisor_role AND u.status = @status
ORDER BY u.id;

-- name: UserInOrganization :one
-- Whether the organization's Members & roles page may act on the user: an administrator, a
-- member, or a requester of one of its clients.
SELECT EXISTS (
  SELECT 1 FROM users u
  LEFT JOIN clients c ON c.id = u.client_id
  WHERE u.id = @user_id
    AND (u.role = @admin_role
         OR c.organization_id = @organization_id
         OR EXISTS (SELECT 1 FROM organization_members m WHERE m.user_id = u.id AND m.organization_id = @organization_id)));

-- name: GetUserIDByEmail :one
SELECT id, role FROM users WHERE email = @email;

-- name: CountOpenTicketsAssignedIn :one
-- A member's open tickets in one organization: they must be handed on before the member leaves.
SELECT count(*)::int FROM tickets
WHERE organization_id = @organization_id AND assignee_id = @user_id AND status = ANY(@open_statuses::text[]);
