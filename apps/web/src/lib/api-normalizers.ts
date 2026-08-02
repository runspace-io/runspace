import type {
  ApiAgentInstallation,
  ApiChannel,
  ApiEvent,
  ApiMessage,
  ApiRun,
  RunStatus,
} from './api-types';
import { resolveMessageIdentity } from './message-identity';
import type { RepositorySummary, TimelineItem, WorkspaceSummary } from './workspace-state';

export function eventToTimelineItem(
  event: ApiEvent,
  viewerID?: string,
  agents: readonly ApiAgentInstallation[] = [],
): TimelineItem | undefined {
  const payload = event.payload;
  if (!payload || typeof payload !== 'object') return undefined;
  if (event.type === 'agent.output') return agentOutputItem(event, payload);
  if (event.type === 'message.created') return messageItem(event, payload, viewerID, agents);
  return undefined;
}

function agentOutputItem(event: ApiEvent, payload: object): TimelineItem | undefined {
  const text = readString(payload as Record<string, unknown>, 'text', 'Text');
  if (!text?.trim()) return undefined;
  return {
    id: event.id,
    author: 'Agent',
    role: 'agent',
    time: formatEventTime(event.occurred_at),
    body: text,
  };
}

function messageItem(
  event: ApiEvent,
  payload: object,
  viewerID: string | undefined,
  agents: readonly ApiAgentInstallation[],
): TimelineItem | undefined {
  const message = payload as Partial<ApiMessage>;
  if (typeof message.body !== 'string' || typeof message.id !== 'string') return undefined;
  const actorType = message.actor_type ?? event.actor_type;
  const actorID = message.actor_id ?? event.actor_id;
  const identity = resolveMessageIdentity(actorID, actorType, viewerID, agents);
  const agentAuthored = Boolean(identity.provider);
  return {
    id: message.id,
    ...identity,
    role: agentAuthored ? 'agent' : actorType === 'user' ? 'human' : 'system',
    activity: actorType === 'activity',
    time: formatEventTime(event.occurred_at),
    body: message.body,
  };
}

export function eventRunContext(event: ApiEvent): {
  runID: string | undefined;
  threadID: string | undefined;
  channelID: string | undefined;
  status: RunStatus | undefined;
} {
  const payload =
    event.payload && typeof event.payload === 'object'
      ? (event.payload as Record<string, unknown>)
      : {};
  return {
    runID: readString(payload, 'run_id', 'RunID', 'id', 'ID'),
    threadID: event.thread_id ?? readString(payload, 'thread_id', 'ThreadID'),
    channelID: event.channel_id ?? readString(payload, 'channel_id', 'ChannelID'),
    status: runStatus(event.type, readString(payload, 'status', 'Status')),
  };
}

const AGENT_TASK_EVENTS = new Set([
  'agent.task.message',
  'agent.task.status',
  'agent.question.asked',
  'agent.question.answered',
]);

// Agent task events announce that a private chat advanced; they never carry the
// message body, because the realtime bus reaches every workspace member. A
// listener that cares must re-read the transcript through the granted endpoint.
export function agentTaskEvent(event: ApiEvent):
  | {
      taskID: string;
      threadID: string | undefined;
      status: string | undefined;
      questionID: string | undefined;
    }
  | undefined {
  if (!AGENT_TASK_EVENTS.has(event.type)) return undefined;
  const payload =
    event.payload && typeof event.payload === 'object'
      ? (event.payload as Record<string, unknown>)
      : {};
  const taskID = readString(payload, 'task_id');
  if (!taskID) return undefined;
  return {
    taskID,
    threadID: event.thread_id ?? readString(payload, 'thread_id'),
    status: readString(payload, 'status'),
    questionID: readString(payload, 'question_id'),
  };
}

export function normalizeWorkspace(value: WorkspaceSummary): WorkspaceSummary {
  const raw = value as WorkspaceSummary & { resource_count?: number; repository_count?: number };
  return {
    ...value,
    resourceCount:
      value.resourceCount ??
      raw.resource_count ??
      value.repositoryCount ??
      raw.repository_count ??
      0,
  };
}

export function normalizeChannel(value: ApiChannel): ApiChannel {
  const ids =
    value.resource_ids ??
    (value.resource_id ? [value.resource_id] : undefined) ??
    value.repository_ids ??
    (value.repository_id ? [value.repository_id] : []);
  const {
    resource_id: _resource,
    resource_ids: _resources,
    repository_id: _legacy,
    ...rest
  } = value;
  return ids.length > 0
    ? { ...rest, repository_ids: ids, repository_id: ids[0]! }
    : { ...rest, repository_ids: [] };
}

export function normalizeResource(value: RepositorySummary): RepositorySummary {
  const raw = value as RepositorySummary & {
    full_name?: string;
    clone_url?: string;
    default_branch?: string;
  };
  return {
    id: value.id,
    fullName: value.fullName ?? raw.full_name ?? value.id,
    cloneURL:
      value.cloneURL ??
      raw.clone_url ??
      `https://github.com/${value.fullName ?? raw.full_name ?? value.id}.git`,
    defaultBranch: value.defaultBranch ?? raw.default_branch ?? 'main',
    provider: value.provider ?? 'github',
  };
}

export const normalizeRepository = normalizeResource;

export function normalizeRun(value: ApiRun): ApiRun {
  const raw = value as ApiRun & Record<string, unknown>;
  const run: ApiRun = {
    id: readString(raw, 'id', 'ID') ?? '',
    workspace_id: readString(raw, 'workspace_id', 'WorkspaceID') ?? '',
    prompt: readString(raw, 'prompt', 'Prompt') ?? '',
    status: (readString(raw, 'status', 'Status') ?? 'queued') as RunStatus,
  };
  run.thread_id = readString(raw, 'thread_id', 'ThreadID');
  run.channel_id = readString(raw, 'channel_id', 'ChannelID');
  run.repository_id = readString(raw, 'repository_id', 'RepositoryID');
  return run;
}

function readString(source: Record<string, unknown>, ...keys: string[]): string | undefined {
  for (const key of keys) {
    const value = source[key];
    if (value !== undefined && value !== null && value !== '') return String(value);
  }
  return undefined;
}

function runStatus(eventType: string, value: unknown): RunStatus | undefined {
  if (eventType === 'run.started') return 'running';
  if (eventType === 'run.stop_requested') return 'stopping';
  if (eventType === 'run.stopped') return 'cancelled';
  if (eventType === 'run.finished') return String(value || 'succeeded') as RunStatus;
  return undefined;
}

function formatEventTime(value: string): string {
  const date = new Date(value);
  return Number.isNaN(date.valueOf())
    ? 'now'
    : date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}
