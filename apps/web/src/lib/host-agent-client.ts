import { normalizeRepository } from './api-normalizers';
import type { ApiRepositoryFile } from './api-types';
import type { RepositorySummary, WorkspaceTreeEntry } from './workspace-state';
import type {
  HostAgentStatus,
  LocalAgentChatSummary,
  LocalAgentInstallation,
  LocalAgentSession,
  LocalRepositoryStatus,
  MirrorResponse,
} from './host-agent-types';

export type * from './host-agent-types';

const hostAgentURL =
  process.env.NEXT_PUBLIC_HOST_AGENT_URL?.replace(/\/$/, '') ?? 'http://127.0.0.1:7799';

export function inspectLocalRepository(path: string): Promise<LocalRepositoryStatus> {
  return hostAgentRequest<LocalRepositoryStatus>('/v1/resources/inspect', { path });
}

export function initializeLocalRepository(path: string): Promise<LocalRepositoryStatus> {
  return hostAgentRequest<LocalRepositoryStatus>('/v1/resources/init', {
    path,
    branch: 'main',
  });
}

export async function suggestLocalPaths(path: string): Promise<string[]> {
  const result = await hostAgentRequest<{ paths: string[] }>('/v1/filesystem/suggest', { path });
  return result.paths ?? [];
}

export async function getHostAgentStatus(): Promise<HostAgentStatus> {
  let response: Response;
  try {
    response = await fetch(`${hostAgentURL}/v1/status`);
  } catch {
    throw new Error('Runspace Host Agent is not reachable.');
  }
  if (!response.ok) throw new Error(`Host Agent returned ${response.status}.`);
  return response.json() as Promise<HostAgentStatus>;
}

export async function discoverLocalAgents(userID: string): Promise<LocalAgentInstallation[]> {
  const result = await hostAgentGet<{ agents: LocalAgentInstallation[] }>(
    '/v1/agents/discover',
    userID,
  );
  return result.agents ?? [];
}

export async function listLocalAgentChats(
  userID: string,
  workspaceID: string,
): Promise<LocalAgentChatSummary[]> {
  const query = new URLSearchParams({ workspace_id: workspaceID });
  const result = await hostAgentGet<{ chats: LocalAgentChatSummary[] }>(
    `/v1/agent-chats?${query.toString()}`,
    userID,
  );
  return result.chats ?? [];
}

export function saveLocalAgentPreference(
  userID: string,
  agentID: string,
  preference: { model: string; permission_mode: 'default' | 'approve' | 'yolo' },
): Promise<typeof preference> {
  return hostAgentRequest<typeof preference>(
    `/v1/agents/${encodeURIComponent(agentID)}/preferences`,
    preference,
    'PUT',
    userID,
  );
}

export function promptLocalAgent(input: {
  userID: string;
  agentID: string;
  resourceID: string;
  threadID: string;
  taskID: string;
  prompt: string;
}): Promise<{ session_id: string; outputs: Array<{ kind: string; text: string }> }> {
  return hostAgentRequest(
    `/v1/agents/${encodeURIComponent(input.agentID)}/prompt`,
    {
      resource_id: input.resourceID,
      thread_id: input.threadID,
      task_id: input.taskID,
      prompt: input.prompt,
    },
    'POST',
    input.userID,
  );
}

export function getLocalAgentSession(input: {
  userID: string;
  agentID: string;
  resourceID: string;
  threadID: string;
  taskID: string;
}): Promise<LocalAgentSession> {
  const query = new URLSearchParams({
    resource_id: input.resourceID,
    thread_id: input.threadID,
    task_id: input.taskID,
  });
  return hostAgentGet(
    `/v1/agents/${encodeURIComponent(input.agentID)}/session?${query.toString()}`,
    input.userID,
  );
}

/** Unblocks the owner's own agent. An empty optionID rejects the request. */
export function answerLocalAgentQuestion(input: {
  userID: string;
  agentID: string;
  resourceID: string;
  threadID: string;
  taskID: string;
  questionID: string;
  optionID: string;
}): Promise<{ status: 'answered' }> {
  return hostAgentRequest(
    `/v1/agents/${encodeURIComponent(input.agentID)}/session/answer`,
    {
      resource_id: input.resourceID,
      thread_id: input.threadID,
      task_id: input.taskID,
      question_id: input.questionID,
      option_id: input.optionID,
    },
    'POST',
    input.userID,
  );
}

export function cancelLocalAgentSession(input: {
  userID: string;
  agentID: string;
  resourceID: string;
  threadID: string;
  taskID: string;
}): Promise<{ status: 'cancelled' }> {
  return hostAgentRequest(
    `/v1/agents/${encodeURIComponent(input.agentID)}/session/cancel`,
    { resource_id: input.resourceID, thread_id: input.threadID, task_id: input.taskID },
    'POST',
    input.userID,
  );
}

export async function listLocalAgentModels(userID: string, agentID: string): Promise<string[]> {
  const result = await hostAgentGet<{ models: string[] }>(
    `/v1/agents/${encodeURIComponent(agentID)}/models`,
    userID,
  );
  return result.models ?? [];
}

export function hostTerminalURL(
  repositoryID: string,
  level: HostAgentStatus['access_level'],
  userID: string,
): string {
  const url = new URL(
    `${hostAgentURL}/v1/resources/${encodeURIComponent(repositoryID)}/terminal?level=${level}`,
  );
  url.searchParams.set('user_id', userID);
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
  return url.toString();
}

export async function listHostRepositoryTree(
  repositoryID: string,
  userID: string,
  path = '',
): Promise<WorkspaceTreeEntry[]> {
  const query = path ? `?path=${encodeURIComponent(path)}` : '';
  const response = await hostAgentGet<{
    entries: Array<{ path: string; kind: string; ignored?: boolean }>;
  }>(`/v1/resources/${encodeURIComponent(repositoryID)}/tree${query}`, userID);
  return response.entries
    .filter((entry) => !entry.ignored && (entry.kind === 'file' || entry.kind === 'directory'))
    .map((entry) => ({ path: entry.path, kind: entry.kind as WorkspaceTreeEntry['kind'] }));
}

export function readHostRepositoryFile(
  repositoryID: string,
  userID: string,
  path: string,
): Promise<ApiRepositoryFile> {
  return hostAgentGet<ApiRepositoryFile>(
    `/v1/resources/${encodeURIComponent(repositoryID)}/file?path=${encodeURIComponent(path)}`,
    userID,
  );
}

export async function connectLocalMirror(input: {
  path: string;
  userID: string;
  workspaceID: string;
}): Promise<RepositorySummary> {
  const configuredGateway = process.env.NEXT_PUBLIC_API_URL ?? '/gateway';
  const gatewayURL = new URL(configuredGateway, window.location.origin)
    .toString()
    .replace(/\/$/, '');
  const payload = await hostAgentRequest<MirrorResponse>('/v1/resources', {
    path: input.path,
    gateway_url: gatewayURL,
    user_id: input.userID,
    workspace_id: input.workspaceID,
  });
  return normalizeRepository(payload.resource ?? payload.repository!);
}

async function hostAgentRequest<T>(
  path: string,
  body: object,
  method = 'POST',
  userID = '',
): Promise<T> {
  let response: Response;
  try {
    response = await fetch(`${hostAgentURL}${path}`, {
      method,
      headers: { 'Content-Type': 'application/json', 'X-User-ID': userID },
      body: JSON.stringify(body),
    });
  } catch {
    throw new Error(
      'Runspace Host Agent is not reachable. Start it on this computer, then try again.',
    );
  }
  const payload = (await response.json().catch(() => ({}))) as Partial<T> & {
    error?: string;
  };
  if (!response.ok) {
    throw new Error(payload.error ?? `Host Agent returned ${response.status}.`);
  }
  return payload as T;
}

async function hostAgentGet<T>(path: string, userID = ''): Promise<T> {
  let response: Response;
  try {
    response = await fetch(`${hostAgentURL}${path}`, { headers: { 'X-User-ID': userID } });
  } catch {
    throw new Error('Runspace Host Agent is not reachable.');
  }
  const payload = (await response.json().catch(() => ({}))) as Partial<T> & {
    error?: string;
  };
  if (!response.ok) {
    throw new Error(payload.error ?? `Host Agent returned ${response.status}.`);
  }
  return payload as T;
}
