-- +goose Up

-- Owner, 17 Nov: each user picks which projects they see on the board, its counts and the
-- ticket lists, per organization. No rows for an organization means every project.
ALTER TABLE projects ADD CONSTRAINT projects_id_organization_key UNIQUE (id, organization_id);

CREATE TABLE user_project_filters (
  user_id         uuid NOT NULL CONSTRAINT user_project_filters_user_id_fkey REFERENCES users (id) ON DELETE CASCADE,
  organization_id uuid NOT NULL,
  project_id      uuid NOT NULL,
  CONSTRAINT user_project_filters_pkey PRIMARY KEY (user_id, organization_id, project_id),
  -- Only projects of that organization; a deleted project leaves every filter.
  CONSTRAINT user_project_filters_project_fkey FOREIGN KEY (project_id, organization_id)
    REFERENCES projects (id, organization_id) ON DELETE CASCADE
);
CREATE INDEX user_project_filters_project_idx ON user_project_filters (project_id, organization_id);

-- +goose Down
DROP TABLE user_project_filters;
ALTER TABLE projects DROP CONSTRAINT projects_id_organization_key;
