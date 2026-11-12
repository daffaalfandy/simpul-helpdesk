-- +goose Up

-- +goose StatementBegin
CREATE FUNCTION set_updated_at() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END
$$;
-- +goose StatementEnd

-- One organisation per install: a single-row settings table.
CREATE TABLE organization (
  id            boolean PRIMARY KEY DEFAULT true CHECK (id),
  name          text NOT NULL CHECK (length(name) BETWEEN 1 AND 200),
  logo_url      text,
  ticket_prefix text NOT NULL DEFAULT 'HD' CHECK (ticket_prefix ~ '^[A-Z][A-Z0-9]{0,9}$'),
  timezone      text NOT NULL DEFAULT 'Asia/Jakarta',
  footer_note   text,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER organization_updated_at BEFORE UPDATE ON organization
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
INSERT INTO organization (name) VALUES ('SimDesk');

CREATE TABLE clients (
  id         uuid PRIMARY KEY DEFAULT uuidv7(),
  name       text NOT NULL UNIQUE CHECK (length(name) BETWEEN 1 AND 200),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER clients_updated_at BEFORE UPDATE ON clients
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE users (
  id                  uuid PRIMARY KEY DEFAULT uuidv7(),
  email               text NOT NULL UNIQUE CHECK (email = lower(email) AND position('@' IN email) > 1),
  name                text NOT NULL CHECK (length(name) BETWEEN 1 AND 200),
  password_hash       text,
  role                text NOT NULL CHECK (role IN ('admin', 'supervisor', 'agent', 'requester')),
  client_id           uuid REFERENCES clients (id) ON DELETE RESTRICT,
  job_title           text,
  status              text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'invited', 'suspended')),
  password_changed_at timestamptz NOT NULL DEFAULT now(),
  last_login_at       timestamptz,
  created_at          timestamptz NOT NULL DEFAULT now(),
  updated_at          timestamptz NOT NULL DEFAULT now(),
  -- Requesters (client staff) belong to exactly one client; vendor staff to none.
  CONSTRAINT users_requester_has_client CHECK ((role = 'requester') = (client_id IS NOT NULL))
);
CREATE INDEX users_client_id_idx ON users (client_id) WHERE client_id IS NOT NULL;
CREATE TRIGGER users_updated_at BEFORE UPDATE ON users
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Server-side sessions. Only the SHA-256 of the cookie token is stored.
CREATE TABLE sessions (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),
  token_hash   bytea NOT NULL UNIQUE,
  user_id      uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  created_at   timestamptz NOT NULL DEFAULT now(),
  last_seen_at timestamptz NOT NULL DEFAULT now(),
  expires_at   timestamptz NOT NULL,
  ip           inet,
  user_agent   text
);
CREATE INDEX sessions_user_id_idx ON sessions (user_id);

CREATE TABLE password_reset_tokens (
  id         uuid PRIMARY KEY DEFAULT uuidv7(),
  token_hash bytea NOT NULL UNIQUE,
  user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL,
  used_at    timestamptz
);
CREATE INDEX password_reset_tokens_user_id_idx ON password_reset_tokens (user_id);

-- Failed sign-in / reset attempts for rate limiting.
-- key is "ip:<addr>", "email:<sha256 hex>" or "reset:<sha256 hex>"; no raw email is stored.
CREATE TABLE login_attempts (
  id  bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  key text NOT NULL,
  at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX login_attempts_key_at_idx ON login_attempts (key, at);

-- Append-only audit trail. No foreign keys: entries must outlive what they describe.
CREATE TABLE audit_log (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  at            timestamptz NOT NULL DEFAULT now(),
  actor_kind    text NOT NULL CHECK (actor_kind IN ('user', 'system', 'git')),
  actor_user_id uuid,
  actor_name    text,
  action        text NOT NULL,
  target_type   text,
  target_id     uuid,
  data          jsonb NOT NULL DEFAULT '{}'::jsonb,
  reason        text,
  ip            inet,
  request_id    text
);
CREATE INDEX audit_log_target_idx ON audit_log (target_type, target_id, at);
CREATE INDEX audit_log_at_idx ON audit_log (at);

-- +goose StatementBegin
CREATE FUNCTION audit_log_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'audit_log is append-only' USING ERRCODE = 'insufficient_privilege';
END
$$;
-- +goose StatementEnd
CREATE TRIGGER audit_log_no_update_delete BEFORE UPDATE OR DELETE ON audit_log
  FOR EACH ROW EXECUTE FUNCTION audit_log_immutable();
CREATE TRIGGER audit_log_no_truncate BEFORE TRUNCATE ON audit_log
  FOR EACH STATEMENT EXECUTE FUNCTION audit_log_immutable();

-- +goose Down
DROP TABLE audit_log;
DROP FUNCTION audit_log_immutable();
DROP TABLE login_attempts;
DROP TABLE password_reset_tokens;
DROP TABLE sessions;
DROP TABLE users;
DROP TABLE clients;
DROP TABLE organization;
DROP FUNCTION set_updated_at();
