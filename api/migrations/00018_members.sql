-- +goose Up

-- Owner, 17 Nov: supervisor and agent are roles per organization. A user is an install-wide
-- administrator, staff (with a membership, and its role, in each organization they work in), or
-- a requester of one client. Existing supervisors and agents become members of the install's
-- organization with the role they had.
CREATE TABLE organization_members (
  user_id         uuid NOT NULL CONSTRAINT organization_members_user_id_fkey REFERENCES users (id) ON DELETE CASCADE,
  organization_id uuid NOT NULL CONSTRAINT organization_members_organization_id_fkey REFERENCES organizations (id),
  role            text NOT NULL CONSTRAINT organization_members_role CHECK (role IN ('supervisor', 'agent')),
  created_at      timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT organization_members_pkey PRIMARY KEY (user_id, organization_id)
);
CREATE INDEX organization_members_organization_idx ON organization_members (organization_id, role);

INSERT INTO organization_members (user_id, organization_id, role)
SELECT u.id, o.id, u.role FROM users u CROSS JOIN organizations o WHERE u.role IN ('supervisor', 'agent');

ALTER TABLE users DROP CONSTRAINT users_role_check;
ALTER TABLE users DISABLE TRIGGER users_updated_at;
UPDATE users SET role = 'staff' WHERE role IN ('supervisor', 'agent');
ALTER TABLE users ENABLE TRIGGER users_updated_at;
ALTER TABLE users
  ADD CONSTRAINT users_role_check CHECK (role IN ('admin', 'staff', 'requester')),
  -- The organization the user last worked in: where the next sign-in starts.
  ADD COLUMN last_organization_id uuid
    CONSTRAINT users_last_organization_id_fkey REFERENCES organizations (id) ON DELETE SET NULL;

-- Only agents of the client's organization are linked to it; leaving the organization unlinks.
DELETE FROM client_agents ca
WHERE NOT EXISTS (SELECT 1 FROM organization_members m
                  WHERE m.user_id = ca.user_id AND m.organization_id = ca.organization_id AND m.role = 'agent');
ALTER TABLE client_agents
  ADD CONSTRAINT client_agents_member_fkey FOREIGN KEY (user_id, organization_id)
    REFERENCES organization_members (user_id, organization_id) ON DELETE CASCADE;

-- +goose Down
-- Only an install where everyone has at most one membership can go back.
-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM organization_members GROUP BY user_id HAVING count(*) > 1) THEN
    RAISE EXCEPTION 'cannot go back to one role per user: someone belongs to several organizations';
  END IF;
END
$$;
-- +goose StatementEnd
ALTER TABLE client_agents DROP CONSTRAINT client_agents_member_fkey;
ALTER TABLE users DROP COLUMN last_organization_id, DROP CONSTRAINT users_role_check;
ALTER TABLE users DISABLE TRIGGER users_updated_at;
UPDATE users u SET role = m.role FROM organization_members m WHERE m.user_id = u.id;
-- Staff without a membership had no role to go back to; agent is the least access.
UPDATE users SET role = 'agent' WHERE role = 'staff';
ALTER TABLE users ENABLE TRIGGER users_updated_at;
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('admin', 'supervisor', 'agent', 'requester'));
DROP TABLE organization_members;
