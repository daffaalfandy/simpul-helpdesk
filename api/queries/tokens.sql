-- Personal access tokens for the MCP server (owner, 17 Nov): one user, one organization.

-- name: CreatePersonalToken :one
INSERT INTO personal_access_tokens (user_id, organization_id, name, token_hash, last_four, access, expires_at)
VALUES (@user_id, @organization_id, @name, @token_hash, @last_four, @access, @expires_at)
RETURNING id, name, last_four, access, created_at, expires_at, last_used_at;

-- name: ListPersonalTokens :many
-- The user's tokens in the organization, newest first.
SELECT id, name, last_four, access, created_at, expires_at, last_used_at
FROM personal_access_tokens
WHERE user_id = @user_id AND organization_id = @organization_id
ORDER BY created_at DESC, id DESC;

-- name: CountLivePersonalTokens :one
-- How many of the user's tokens in the organization haven't expired.
SELECT count(*)::int
FROM personal_access_tokens
WHERE user_id = @user_id AND organization_id = @organization_id AND expires_at > @now;

-- name: DeletePersonalToken :one
-- Revokes one of the user's tokens in the organization.
DELETE FROM personal_access_tokens
WHERE id = @id AND user_id = @user_id AND organization_id = @organization_id
RETURNING id, access, expires_at;

-- name: DeleteUserTokens :exec
-- Every token of a suspended user.
DELETE FROM personal_access_tokens WHERE user_id = @user_id;

-- name: DeleteTokensOutsideOrganization :exec
-- A former administrator keeps tokens only for the organization they now belong to; tokens made
-- elsewhere would come back to life if they joined that organization later.
DELETE FROM personal_access_tokens WHERE user_id = @user_id AND organization_id <> @organization_id;

-- name: DeleteMemberTokens :exec
-- A removed member's tokens for that organization.
DELETE FROM personal_access_tokens WHERE user_id = @user_id AND organization_id = @organization_id;

-- name: GetTokenPrincipal :one
-- The token and its user, for signing a request in.
SELECT t.id AS token_id, t.organization_id, t.access, t.expires_at, t.last_used_at,
       u.id AS user_id, u.email, u.name, u.role, u.client_id, c.name AS client_name,
       c.organization_id AS client_organization_id, u.job_title, u.status, u.last_organization_id
FROM personal_access_tokens t
JOIN users u ON u.id = t.user_id
LEFT JOIN clients c ON c.id = u.client_id
WHERE t.token_hash = @token_hash;

-- name: TouchPersonalToken :exec
UPDATE personal_access_tokens SET last_used_at = now() WHERE id = @id;
