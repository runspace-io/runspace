CREATE TABLE IF NOT EXISTS resource_connections (
  id text PRIMARY KEY,
  workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  plugin_id text NOT NULL,
  title text NOT NULL,
  placement text NOT NULL,
  auth_method text NOT NULL,
  access_mode text NOT NULL,
  owner_id text NOT NULL,
  config jsonb NOT NULL DEFAULT '{}'::jsonb,
  sealed_credential bytea NOT NULL,
  capabilities jsonb NOT NULL DEFAULT '[]'::jsonb,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL
);

CREATE INDEX IF NOT EXISTS resource_connections_workspace
  ON resource_connections(workspace_id, updated_at DESC);
