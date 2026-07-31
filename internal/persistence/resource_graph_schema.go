package persistence

const graphSchema = `
CREATE TABLE IF NOT EXISTS resource_graph_nodes (id text PRIMARY KEY, workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE, kind text NOT NULL, type text NOT NULL, title text NOT NULL, summary text NOT NULL DEFAULT '', external_ref text NOT NULL DEFAULT '', owner_id text NOT NULL, metadata jsonb NOT NULL DEFAULT '{}'::jsonb, created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL);
CREATE INDEX IF NOT EXISTS resource_graph_nodes_workspace_kind ON resource_graph_nodes(workspace_id,kind,updated_at DESC);
CREATE INDEX IF NOT EXISTS resource_graph_nodes_metadata ON resource_graph_nodes USING gin(metadata);
CREATE TABLE IF NOT EXISTS resource_graph_edges (id text PRIMARY KEY, workspace_id text NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE, from_id text NOT NULL REFERENCES resource_graph_nodes(id) ON DELETE CASCADE, to_id text NOT NULL REFERENCES resource_graph_nodes(id) ON DELETE CASCADE, relation text NOT NULL, created_by text NOT NULL, metadata jsonb NOT NULL DEFAULT '{}'::jsonb, created_at timestamptz NOT NULL);
CREATE INDEX IF NOT EXISTS resource_graph_edges_from ON resource_graph_edges(workspace_id,from_id,relation);
CREATE INDEX IF NOT EXISTS resource_graph_edges_to ON resource_graph_edges(workspace_id,to_id,relation);
INSERT INTO resource_graph_nodes (id,workspace_id,kind,type,title,owner_id,metadata,created_at,updated_at)
SELECT 'resource:'||id,workspace_id,'resource',provider,full_name,created_by,jsonb_build_object('entity_id',id,'provider',provider),created_at,created_at FROM repositories ON CONFLICT (id) DO NOTHING;
INSERT INTO resource_graph_nodes (id,workspace_id,kind,type,title,owner_id,metadata,created_at,updated_at)
SELECT 'discussion:'||id,workspace_id,'discussion','thread',title,created_by,jsonb_build_object('entity_id',id,'thread_id',id,'channel_id',COALESCE(channel_id,'')),created_at,created_at FROM threads ON CONFLICT (id) DO NOTHING;
INSERT INTO resource_graph_nodes (id,workspace_id,kind,type,title,owner_id,metadata,created_at,updated_at)
SELECT 'task:'||id,workspace_id,'task','agent_work',title,owner_id,jsonb_build_object('entity_id',id,'thread_id',thread_id,'agent_id',agent_id,'resource_id',resource_id,'status',status),created_at,updated_at FROM agent_tasks ON CONFLICT (id) DO NOTHING;
INSERT INTO resource_graph_edges (id,workspace_id,from_id,to_id,relation,created_by,created_at)
SELECT 'edge:task-discussion:'||id,workspace_id,'task:'||id,'discussion:'||thread_id,'discussed_in',owner_id,created_at FROM agent_tasks ON CONFLICT (id) DO NOTHING;
INSERT INTO resource_graph_edges (id,workspace_id,from_id,to_id,relation,created_by,created_at)
SELECT 'edge:task-resource:'||id,workspace_id,'task:'||id,'resource:'||resource_id,'uses',owner_id,created_at FROM agent_tasks WHERE EXISTS (SELECT 1 FROM repositories r WHERE r.id=agent_tasks.resource_id) ON CONFLICT (id) DO NOTHING;`
