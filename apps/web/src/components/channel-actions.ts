import type { Dispatch, SetStateAction } from 'react';
import type {
  ApiAgentInstallation,
  ApiChannel,
  ApiRun,
  ApiRunOutput,
  WorkspaceApiClient,
} from '@/lib/api-client';
import { resolveMessageIdentity } from '@/lib/message-identity';
import {
  parseGithubRepository,
  type RepositorySummary,
  type TimelineItem,
  type WorkspaceSummary,
} from '@/lib/workspace-state';
import type { ChannelDraft, ChannelSettingsDraft } from './channel-model';

type ChannelActionState = {
  setChannels: Dispatch<SetStateAction<ApiChannel[]>>;
  setRepositories: Dispatch<SetStateAction<RepositorySummary[]>>;
  setThreadID: (id: string | undefined) => void;
  setActiveChannelID?: (id: string | undefined) => void;
  setTimeline?: (items: TimelineItem[]) => void;
  setActiveRun?: (run: ApiRun | undefined) => void;
  setFormError: (value: string | undefined) => void;
};

export async function createChannelRequest(input: {
  api: WorkspaceApiClient;
  workspace: WorkspaceSummary | undefined;
  repositories: readonly RepositorySummary[];
  draft: ChannelDraft;
  state: ChannelActionState;
}): Promise<boolean> {
  const { api, workspace, repositories, draft, state } = input;
  if (!workspace || !draft.name.trim()) return false;
  try {
    const repositoryIDs = await resolveRepositories(api, workspace.id, repositories, draft, state);
    const config = agentConfig(draft.agentProtocol, draft.agentCommand);
    const channel = await api.createChannel(workspace.id, draft.name.trim(), draft.parentID, {
      repositoryIDs,
      config,
    });
    state.setChannels((current) => [...current, channel]);
    state.setActiveChannelID?.(channel.id);
    state.setTimeline?.([]);
    const thread = await api.createThread(workspace.id, channel.name, channel.id);
    state.setThreadID(thread.id);
    state.setFormError(undefined);
    return true;
  } catch (error) {
    state.setFormError(repositoryError(error, 'Unable to create channel.'));
    return false;
  }
}

export async function updateChannelRequest(input: {
  api: WorkspaceApiClient;
  workspace: WorkspaceSummary | undefined;
  channel: ApiChannel;
  repositories: readonly RepositorySummary[];
  draft: ChannelSettingsDraft;
  state: ChannelActionState;
}): Promise<boolean> {
  const { api, workspace, channel, repositories, draft, state } = input;
  if (!workspace || !draft.name.trim()) return false;
  try {
    const repositoryIDs = await resolveRepositories(
      api,
      workspace.id,
      repositories,
      { ...draft, parentID: channel.parent_id ?? '' },
      state,
    );
    const config = agentConfig(draft.agentProtocol, draft.agentCommand);
    const updated = await api.updateChannel(workspace.id, channel.id, {
      name: draft.name.trim(),
      resource_id: repositoryIDs[0] ?? '',
      resource_ids: repositoryIDs,
      config,
    });
    state.setChannels((current) =>
      current.map((item) => (item.id === updated.id ? updated : item)),
    );
    state.setFormError(undefined);
    return true;
  } catch (error) {
    state.setFormError(repositoryError(error, 'Unable to update channel settings.'));
    return false;
  }
}

function agentConfig(protocol: ChannelDraft['agentProtocol'], command: string) {
  if (protocol === 'mock') return { agent: { protocol: 'mock' } };
  if (protocol === 'acp' && command.trim()) {
    if (command.startsWith('local_agent_')) {
      return {
        agent: {
          protocol: 'acp',
          placement: 'host',
          installation_id: command.trim(),
        },
      };
    }
    return { agent: { protocol: 'acp', command: command.trim() } };
  }
  return {};
}

async function resolveRepositories(
  api: WorkspaceApiClient,
  workspaceID: string,
  repositories: readonly RepositorySummary[],
  draft: ChannelDraft,
  state: ChannelActionState,
): Promise<string[]> {
  const selected = [...new Set(draft.repositoryIDs)];
  const cloneable = repositoryIDsRequiringClone(repositories, draft.repositoryIDs);
  if (cloneable.length > 0) {
    await Promise.all(cloneable.map((id) => api.prepareResource(workspaceID, id)));
  }
  if (!draft.repositoryURL.trim()) return selected;
  const parsed = parseGithubRepository(draft.repositoryURL);
  if (!parsed) throw new Error('Invalid Git resource URL');
  const existing = repositories.find((repository) => repository.fullName === parsed.fullName);
  if (existing) {
    await api.prepareResource(workspaceID, existing.id);
    return [...new Set([...selected, existing.id])];
  }
  const connected = await api.connectResource(workspaceID, parsed);
  state.setRepositories((current) => [...current, connected]);
  await api.prepareResource(workspaceID, connected.id);
  return [...new Set([...selected, connected.id])];
}

export function repositoryIDsRequiringClone(
  repositories: readonly RepositorySummary[],
  repositoryIDs: readonly string[],
): string[] {
  const selected = new Set(repositoryIDs);
  return repositories
    .filter(
      (repository) =>
        selected.has(repository.id) &&
        repository.provider !== 'mirror' &&
        repository.provider !== 'folder',
    )
    .map((repository) => repository.id);
}

function repositoryError(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message.trim()) return `${fallback} ${error.message}`;
  return fallback;
}

export function openChannelRequest(input: {
  api: WorkspaceApiClient;
  workspace: WorkspaceSummary | undefined;
  channel: ApiChannel;
  setActiveChannelID: (id: string | undefined) => void;
  setThreadID: (id: string | undefined) => void;
  setTimeline: (items: TimelineItem[]) => void;
  setActiveRun: (run: ApiRun | undefined) => void;
  setFormError: (value: string | undefined) => void;
}) {
  const {
    api,
    workspace,
    channel,
    setActiveChannelID,
    setThreadID,
    setTimeline,
    setActiveRun,
    setFormError,
  } = input;
  if (!workspace) return;
  setActiveChannelID(channel.id);
  void api
    .listThreads(workspace.id)
    .then(async (threads) => {
      const existing = threads.find((thread) => thread.channel_id === channel.id);
      const thread = existing ?? (await api.createThread(workspace.id, channel.name, channel.id));
      setThreadID(thread.id);
      const history = await loadChannelHistory(api, workspace.id, thread.id);
      setTimeline(history.timeline);
      setActiveRun(history.latestRun);
    })
    .catch(() => setFormError('Unable to open channel. Please retry.'));
}

async function loadChannelHistory(
  api: WorkspaceApiClient,
  workspaceID: string,
  threadID: string,
): Promise<{ timeline: TimelineItem[]; latestRun: ApiRun | undefined }> {
  const [messages, runs, agents] = await Promise.all([
    api.listMessages(workspaceID, threadID),
    api.listRuns(threadID, workspaceID),
    api.listWorkspaceAgents(workspaceID),
  ]);
  const outputGroups = await Promise.all(runs.map((run) => api.listRunOutputs(run.id)));
  const entries = [
    ...messages.map((message) => ({
      occurredAt: message.created_at,
      item: messageToTimelineItem(message, api.actorID, agents),
    })),
    ...outputGroups.flat().map((output) => ({
      occurredAt: output.created_at,
      item: outputToTimelineItem(output),
    })),
  ];
  entries.sort((left, right) => Date.parse(left.occurredAt) - Date.parse(right.occurredAt));
  return { timeline: entries.map((entry) => entry.item), latestRun: runs.at(-1) };
}

function outputToTimelineItem(output: ApiRunOutput): TimelineItem {
  return {
    id: output.id,
    author: 'Agent',
    role: 'agent',
    time: new Date(output.created_at).toLocaleTimeString([], {
      hour: '2-digit',
      minute: '2-digit',
    }),
    body: output.text,
  };
}

export function messageToTimelineItem(
  message: {
    id: string;
    actor_id: string;
    actor_type: string;
    body: string;
    created_at: string;
  },
  viewerID?: string,
  agents: readonly ApiAgentInstallation[] = [],
): TimelineItem {
  const identity = resolveMessageIdentity(message.actor_id, message.actor_type, viewerID, agents);
  const agentAuthored = Boolean(identity.provider);
  return {
    id: message.id,
    ...identity,
    role: agentAuthored ? 'agent' : message.actor_type === 'user' ? 'human' : 'system',
    activity: message.actor_type === 'activity',
    time: new Date(message.created_at).toLocaleTimeString([], {
      hour: '2-digit',
      minute: '2-digit',
    }),
    body: message.body,
  };
}

export async function sendMessageRequest(input: {
  api: WorkspaceApiClient;
  workspace: WorkspaceSummary | undefined;
  threadID: string | undefined;
  draft: string;
  setDraft: (value: string) => void;
  setTimeline: Dispatch<SetStateAction<TimelineItem[]>>;
  setError: (value: string | undefined) => void;
}): Promise<boolean> {
  const { api, workspace, threadID, draft, setDraft, setTimeline, setError } = input;
  const body = draft.trim();
  if (!body) return false;
  if (!threadID || !workspace) {
    setError('Select a channel before sending a message.');
    return false;
  }
  setDraft('');
  try {
    const message = await api.createMessage(workspace.id, threadID, body);
    setTimeline((current) =>
      current.some((item) => item.id === message.id)
        ? current
        : [...current, messageToTimelineItem(message, api.actorID)],
    );
    return true;
  } catch {
    setDraft(body);
    setError('Message could not be sent. Your draft was restored.');
    return false;
  }
}
