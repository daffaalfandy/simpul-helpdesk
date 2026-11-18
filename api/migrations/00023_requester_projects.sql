-- +goose Up

-- Requesters per project (10b, owner 18 Nov): each requester reports on, and sees, the tickets of
-- the projects of their client they are linked to, plus the tickets they reported themselves.
-- Lets a link name a requester together with their client.
ALTER TABLE users ADD CONSTRAINT users_id_client_key UNIQUE (id, client_id);

CREATE TABLE requester_projects (
  user_id         uuid NOT NULL,
  project_id      uuid NOT NULL,
  client_id       uuid NOT NULL,
  organization_id uuid NOT NULL,
  created_at      timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT requester_projects_pkey PRIMARY KEY (user_id, project_id),
  -- Only a requester of the client (only requesters carry a client_id), only that client's
  -- projects, and the client's own organization.
  CONSTRAINT requester_projects_user_fkey FOREIGN KEY (user_id, client_id)
    REFERENCES users (id, client_id) ON DELETE CASCADE,
  CONSTRAINT requester_projects_project_fkey FOREIGN KEY (project_id, client_id)
    REFERENCES projects (id, client_id) ON DELETE CASCADE,
  CONSTRAINT requester_projects_client_fkey FOREIGN KEY (client_id, organization_id)
    REFERENCES clients (id, organization_id) ON DELETE CASCADE
);
CREATE INDEX requester_projects_project_idx ON requester_projects (project_id);

-- Nobody loses access: every requester reports on every project of their client, hidden ones too.
INSERT INTO requester_projects (user_id, project_id, client_id, organization_id)
SELECT u.id, p.id, p.client_id, p.organization_id
FROM users u
JOIN projects p ON p.client_id = u.client_id
WHERE u.role = 'requester';

-- +goose Down
DROP TABLE requester_projects;
ALTER TABLE users DROP CONSTRAINT users_id_client_key;
