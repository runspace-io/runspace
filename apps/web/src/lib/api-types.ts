export type ApiEvent = {
  id: string;
  type: string;
  workspace_id: string;
  repository_id?: string;
  channel_id?: string;
  thread_id?: string;
  actor_id: string;
  actor_type: string;
  occurred_at: string;
  payload: unknown;
};

export type ApiMessage = {
  id: string;
  thread_id: string;
  actor_id: string;
  actor_type: string;
  body: string;
  created_at: string;
};

export type ApiAgentInstallation = {
  owner_id: string;
  id: string;
  registry_id: string;
  name: string;
  description?: string;
  protocol: 'acp';
  placement: 'host';
  status: string;
  capabilities: string[] | null;
  updated_at: string;
};

export type ApiThread = {
  id: string;
  workspace_id: string;
  channel_id?: string;
  /** Set when this thread is a subthread anchored to a message, not a channel's root thread. */
  parent_thread_id?: string;
  parent_message_id?: string;
  visibility: 'public' | 'private';
  title: string;
  created_by: string;
};

export type ApiChannel = {
  id: string;
  workspace_id: string;
  name: string;
  parent_id?: string;
  resource_id?: string;
  resource_ids?: string[];
  repository_id?: string;
  repository_ids?: string[];
  config?: Record<string, unknown>;
};

export type ApiSecretMetadata = {
  name: string;
  updated_at: string;
  source_channel_id?: string;
  inherited?: boolean;
};
export type ApiRepositoryFile = { path: string; size: number; content: string };
export type ApiRepositorySync = {
  status: {
    state: string;
    error?: string;
    localFiles: number;
    needFiles: number;
    needBytes: number;
  };
};
export type ApiRepositoryChange = {
  path: string;
  status: 'added' | 'modified' | 'deleted' | 'renamed' | 'untracked';
};
export type ApiRepositoryDiff = {
  path: string;
  original: string;
  modified: string;
};
export type ApiMember = {
  workspace_id: string;
  user_id: string;
  role: 'owner' | 'admin' | 'member' | 'viewer';
  created_at: string;
};

export type ApiTaskGrant = {
  task_id: string;
  workspace_id: string;
  owner_id: string;
  agent_id: string;
  principal_id: string;
  role: 'viewer' | 'contributor' | 'operator' | 'approver';
  permissions: string[];
  expires_at?: string;
  created_at: string;
  updated_at: string;
};

export type ApiAgentTask = {
  id: string;
  workspace_id: string;
  thread_id: string;
  owner_id: string;
  agent_id: string;
  resource_id: string;
  title: string;
  status: 'ready' | 'running' | 'waiting_approval' | 'completed' | 'failed' | 'cancelled';
  created_at: string;
  updated_at: string;
};

export type ApiAgentTaskMessage = {
  id: string;
  role: 'user' | 'agent';
  /** "tool_call" is a command the agent ran, rendered as terminal activity. */
  kind?: string | undefined;
  body: string;
  created_at: string;
};

export type ApiInvitation = {
  id: string;
  workspace_id: string;
  role: 'admin' | 'member' | 'viewer';
  created_by: string;
  expires_at: string;
  accepted_by?: string;
  accepted_at?: string;
  created_at: string;
};

/** What a link holder may see before deciding to join. */
export type ApiInvitationPreview = {
  workspace_id: string;
  workspace_name: string;
  role: ApiInvitation['role'];
  invited_by: string;
};

export type ApiQuestionOption = {
  id: string;
  name: string;
  kind: string;
};

/** A permission request the agent is blocked on until someone answers it. */
export type ApiTaskQuestion = {
  id: string;
  task_id: string;
  title: string;
  options: ApiQuestionOption[];
  status: 'open' | 'answered' | 'cancelled';
  answered_by?: string;
  answered_option?: string;
  asked_at: string;
  updated_at: string;
};

export type ApiGraphKind =
  'resource' | 'task' | 'artifact' | 'action' | 'discussion' | 'identity' | 'policy' | 'event';

export type ApiGraphNode = {
  id: string;
  workspace_id: string;
  kind: ApiGraphKind;
  type: string;
  title: string;
  summary?: string;
  external_ref?: string;
  owner_id: string;
  metadata?: Record<string, unknown>;
  created_at: string;
  updated_at: string;
};

export type ApiGraphEdge = {
  id: string;
  workspace_id: string;
  from_id: string;
  to_id: string;
  relation: string;
  created_by: string;
  metadata?: Record<string, unknown>;
  created_at: string;
};

export type ApiGraphContext = {
  node: ApiGraphNode;
  incoming: ApiGraphEdge[];
  outgoing: ApiGraphEdge[];
};

export type ApiResourcePluginCapability = {
  id: string;
  label: string;
  description: string;
  mode: 'query' | 'action';
  risk: 'read' | 'write' | 'destructive';
};

export type ApiResourcePlugin = {
  id: 'github' | 'digitalocean' | 'postgresql';
  name: string;
  description: string;
  resource_type: string;
  placements: Array<'runspace' | 'connector'>;
  auth_methods: Array<{
    id: string;
    label: string;
    secret_label: string;
    placeholder: string;
  }>;
  capabilities: ApiResourcePluginCapability[];
};

export type ApiResourceConnection = {
  id: string;
  workspace_id: string;
  plugin_id: ApiResourcePlugin['id'];
  title: string;
  placement: 'runspace' | 'connector';
  auth_method: string;
  access_mode: 'read' | 'manage' | 'full';
  owner_id: string;
  capabilities: ApiResourcePluginCapability[];
  created_at: string;
  updated_at: string;
};

export type ApiUIDocument = {
  version: 'runspace.ui/v1';
  title: string;
  layout: ApiUIComponentNode;
};

export type ApiUIComponentNode = {
  type: string;
  props?: Record<string, unknown>;
  children?: ApiUIComponentNode[];
};

export type RunStatus = 'queued' | 'running' | 'stopping' | 'succeeded' | 'failed' | 'cancelled';

export type ApiRun = {
  id: string;
  workspace_id: string;
  thread_id?: string | undefined;
  channel_id?: string | undefined;
  repository_id?: string | undefined;
  prompt: string;
  status: RunStatus;
};

export type ApiRunOutput = {
  id: string;
  run_id: string;
  kind: string;
  text: string;
  sequence: number;
  created_at: string;
};

export type ApiPublishRequest = {
  repository_id: string;
  branch: string;
  base: string;
  commit_message: string;
  title: string;
  body: string;
};

export type ApiPublishResult = {
  id: string;
  branch: { name: string; sha: string };
  commit_sha: string;
  pull_request: { number: number; url: string };
  created_at: string;
};
