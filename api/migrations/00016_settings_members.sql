-- +goose Up

-- Settings › General (M9): an uploaded logo replaces the free-text logo_url, which nothing set.
-- setup_completed_at marks the install as set up: first-run setup is open only while it is
-- NULL, so demoting or suspending admins later never opens it again. Installs that already
-- have an admin count as set up.
ALTER TABLE organization
  DROP COLUMN logo_url,
  ADD COLUMN logo_key text CONSTRAINT organization_logo_key CHECK (logo_key ~ '^[0-9a-f]{32}$'),
  ADD COLUMN logo_content_type text
    CONSTRAINT organization_logo_content_type CHECK (logo_content_type IN ('image/png', 'image/jpeg', 'image/webp')),
  ADD COLUMN logo_updated_at timestamptz,
  ADD COLUMN setup_completed_at timestamptz,
  ADD CONSTRAINT organization_logo_complete CHECK (
    (logo_key IS NULL) = (logo_content_type IS NULL) AND (logo_key IS NULL) = (logo_updated_at IS NULL));
UPDATE organization SET setup_completed_at = now() WHERE EXISTS (SELECT 1 FROM users WHERE role = 'admin');

-- Invitations (M9): an emailed, single-use link to set a password, stored hashed like reset
-- tokens. Resending invalidates the earlier ones.
CREATE TABLE user_invites (
  id         uuid PRIMARY KEY DEFAULT uuidv7(),
  token_hash bytea NOT NULL CONSTRAINT user_invites_token_hash_key UNIQUE,
  user_id    uuid NOT NULL CONSTRAINT user_invites_user_id_fkey REFERENCES users (id) ON DELETE CASCADE,
  invited_by uuid CONSTRAINT user_invites_invited_by_fkey REFERENCES users (id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL,
  used_at    timestamptz
);
CREATE INDEX user_invites_user_id_idx ON user_invites (user_id, created_at);

-- +goose Down
DROP TABLE user_invites;
ALTER TABLE organization
  DROP CONSTRAINT organization_logo_complete,
  DROP COLUMN setup_completed_at,
  DROP COLUMN logo_updated_at,
  DROP COLUMN logo_content_type,
  DROP COLUMN logo_key,
  ADD COLUMN logo_url text;
