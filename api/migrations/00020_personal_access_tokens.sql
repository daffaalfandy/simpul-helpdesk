-- +goose Up

-- Owner, 17 Nov: staff connect an AI client to the MCP server with a personal token. A token
-- acts as its user in the one organization it was made in, read only or read and write, until
-- it expires or is revoked. Only its hash is stored; last_four helps the user tell tokens apart.
CREATE TABLE personal_access_tokens (
  id              uuid PRIMARY KEY DEFAULT uuidv7(),
  user_id         uuid NOT NULL CONSTRAINT personal_access_tokens_user_id_fkey REFERENCES users (id) ON DELETE CASCADE,
  organization_id uuid NOT NULL CONSTRAINT personal_access_tokens_organization_id_fkey REFERENCES organizations (id),
  name            text NOT NULL CONSTRAINT personal_access_tokens_name CHECK (length(name) BETWEEN 1 AND 100),
  token_hash      bytea NOT NULL CONSTRAINT personal_access_tokens_token_hash_key UNIQUE,
  last_four       text NOT NULL CONSTRAINT personal_access_tokens_last_four CHECK (length(last_four) = 4),
  access          text NOT NULL CONSTRAINT personal_access_tokens_access CHECK (access IN ('read', 'write')),
  created_at      timestamptz NOT NULL DEFAULT now(),
  expires_at      timestamptz NOT NULL,
  last_used_at    timestamptz,
  CONSTRAINT personal_access_tokens_expiry CHECK (expires_at > created_at)
);
CREATE INDEX personal_access_tokens_user_idx ON personal_access_tokens (user_id, organization_id, created_at);

-- +goose Down
DROP TABLE personal_access_tokens;
