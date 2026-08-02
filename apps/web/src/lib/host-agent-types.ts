import type { RepositorySummary } from './workspace-state';

export type MirrorResponse = {
  resource: RepositorySummary;
  repository?: RepositorySummary;
  sync: {
    status: {
      state: string;
      error?: string;
    };
  };
};

export type LocalRepositoryStatus = {
  path: string;
  git: boolean;
  origin?: string;
  branch?: string;
  has_remote: boolean;
  can_connect: boolean;
};

export type HostAgentStatus = {
  status: string;
  access_level: 'user' | 'administrator';
  elevated: boolean;
};

export type LocalAgentInstallation = {
  id: string;
  registry_id: string;
  name: string;
  description: string;
  protocol: 'acp';
  placement: 'host';
  status: 'ready' | 'adapter_required';
  capabilities: string[];
  model?: string;
  permission_mode?: 'default' | 'approve' | 'yolo';
};

export type LocalTaskMessage = {
  id: string;
  role: 'user' | 'agent';
  body: string;
  created_at: string;
};

/** A permission request the agent is blocked on until someone answers. */
export type LocalPendingQuestion = {
  id: string;
  title: string;
  options: Array<{ id: string; name: string; kind: string }>;
  asked_at: string;
};

export type LocalAgentSession = {
  id: string;
  title: string;
  agent_id: string;
  resource_id: string;
  thread_id: string;
  status: 'draft' | 'ready' | 'running' | 'waiting_approval' | 'completed' | 'failed' | 'cancelled';
  pause_support: 'native' | 'process-suspend' | 'cancel-only';
  messages: LocalTaskMessage[];
  question?: LocalPendingQuestion | undefined;
  updated_at?: string;
};

export type LocalAgentChatSummary = {
  id: string;
  title: string;
  agent_id: string;
  resource_id: string;
  status: LocalAgentSession['status'];
  updated_at?: string;
};
