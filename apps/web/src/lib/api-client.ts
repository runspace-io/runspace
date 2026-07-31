import type {
  ApiAgentInstallation,
  ApiChannel,
  ApiMember,
  ApiMessage,
  ApiPublishRequest,
  ApiPublishResult,
  ApiRun,
  ApiRunOutput,
  ApiSecretMetadata,
  ApiThread,
} from './api-types';
export type * from './api-types';
import type { WorkspaceSummary } from './workspace-state';
import { normalizeChannel, normalizeRun, normalizeWorkspace } from './api-normalizers';
import { GraphApiClient } from './graph-api-client';
export { eventToTimelineItem } from './api-normalizers';
export { ApiError, type ApiClientOptions } from './api-transport';

export class WorkspaceApiClient extends GraphApiClient {
  public realtimeURL(_workspaceID: string): string {
    const realtimeBase = process.env.NEXT_PUBLIC_WS_API_URL ?? this.baseURL;
    const url = new URL(`${realtimeBase}/realtime`, window.location.origin);
    url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
    return url.toString();
  }

  public listWorkspaces(): Promise<WorkspaceSummary[]> {
    return this.request<{ workspaces: WorkspaceSummary[] }>('/workspaces').then((data) =>
      data.workspaces.map(normalizeWorkspace),
    );
  }

  public listWorkspaceAgents(workspaceID: string): Promise<ApiAgentInstallation[]> {
    return this.request<{ agents: ApiAgentInstallation[] | null }>(
      `/workspaces/${encodeURIComponent(workspaceID)}/agents`,
    ).then((data) => data.agents ?? []);
  }

  public publishRun(
    workspaceID: string,
    runID: string,
    request: ApiPublishRequest,
  ): Promise<ApiPublishResult> {
    return this.request<ApiPublishResult>(
      `/workspaces/${encodeURIComponent(workspaceID)}/runs/${encodeURIComponent(runID)}/publish`,
      { method: 'POST', body: JSON.stringify(request) },
    );
  }

  public createWorkspace(name: string): Promise<WorkspaceSummary> {
    return this.request<WorkspaceSummary>('/workspaces', {
      method: 'POST',
      body: JSON.stringify({ name }),
    }).then(normalizeWorkspace);
  }

  public listMessages(workspaceID: string, threadID: string): Promise<ApiMessage[]> {
    return this.request<{ messages: ApiMessage[] | null }>(
      `/threads/${encodeURIComponent(threadID)}/messages?workspace_id=${encodeURIComponent(workspaceID)}`,
    ).then((data) => data.messages ?? []);
  }

  public listThreads(workspaceID: string): Promise<ApiThread[]> {
    return this.request<{ threads: ApiThread[] }>(
      `/workspaces/${encodeURIComponent(workspaceID)}/threads`,
    ).then((data) => data.threads);
  }

  public createThread(workspaceID: string, title: string, channelID = ''): Promise<ApiThread> {
    return this.request<ApiThread>(`/workspaces/${encodeURIComponent(workspaceID)}/threads`, {
      method: 'POST',
      body: JSON.stringify({ title, channel_id: channelID }),
    });
  }

  public listChannels(workspaceID: string): Promise<ApiChannel[]> {
    return this.request<{ channels: ApiChannel[] }>(
      `/workspaces/${encodeURIComponent(workspaceID)}/channels`,
    ).then((data) => data.channels.map(normalizeChannel));
  }

  public createChannel(
    workspaceID: string,
    name: string,
    parentID = '',
    options: {
      repositoryID?: string;
      repositoryIDs?: readonly string[];
      config?: Record<string, unknown>;
    } = {},
  ): Promise<ApiChannel> {
    const repositoryID = options.repositoryID ?? options.repositoryIDs?.[0] ?? '';
    const repositoryIDs = options.repositoryIDs ?? (repositoryID ? [repositoryID] : []);
    const config = options.config ?? {};
    return this.request<ApiChannel>(`/workspaces/${encodeURIComponent(workspaceID)}/channels`, {
      method: 'POST',
      body: JSON.stringify({
        name,
        parent_id: parentID,
        resource_id: repositoryID,
        resource_ids: repositoryIDs,
        config,
      }),
    }).then(normalizeChannel);
  }

  public updateChannel(
    workspaceID: string,
    channelID: string,
    patch: {
      name?: string;
      resource_id?: string;
      resource_ids?: readonly string[];
      config?: Record<string, unknown>;
    },
  ): Promise<ApiChannel> {
    return this.request<ApiChannel>(
      `/workspaces/${encodeURIComponent(workspaceID)}/channels/${encodeURIComponent(channelID)}`,
      { method: 'PATCH', body: JSON.stringify(patch) },
    ).then(normalizeChannel);
  }

  public listChannelSecrets(channelID: string): Promise<ApiSecretMetadata[]> {
    return this.request<{ secrets: ApiSecretMetadata[] }>(
      `/channels/${encodeURIComponent(channelID)}/secrets`,
    ).then((data) => data.secrets);
  }

  public setChannelSecret(channelID: string, name: string, value: string): Promise<void> {
    return this.request<void>(
      `/channels/${encodeURIComponent(channelID)}/secrets/${encodeURIComponent(name)}`,
      { method: 'PUT', body: JSON.stringify({ value }) },
    );
  }

  public deleteChannelSecret(channelID: string, name: string): Promise<void> {
    return this.request<void>(
      `/channels/${encodeURIComponent(channelID)}/secrets/${encodeURIComponent(name)}`,
      { method: 'DELETE' },
    );
  }

  public listMembers(workspaceID: string): Promise<ApiMember[]> {
    return this.request<{ members: ApiMember[] | null }>(
      `/workspaces/${encodeURIComponent(workspaceID)}/members`,
    ).then((data) => data.members ?? []);
  }

  public addMember(workspaceID: string, userID: string, role = 'member'): Promise<ApiMember> {
    return this.request<ApiMember>(`/workspaces/${encodeURIComponent(workspaceID)}/members`, {
      method: 'POST',
      body: JSON.stringify({ user_id: userID, role }),
    });
  }

  public createRun(
    threadID: string,
    input: { runID: string; workspaceID: string; repositoryID: string; prompt: string },
  ): Promise<ApiRun> {
    return this.request<ApiRun>(`/threads/${encodeURIComponent(threadID)}/runs`, {
      method: 'POST',
      body: JSON.stringify({
        run_id: input.runID,
        workspace_id: input.workspaceID,
        repository: input.repositoryID,
        prompt: input.prompt,
      }),
    }).then(normalizeRun);
  }

  public listRuns(threadID: string, workspaceID: string): Promise<ApiRun[]> {
    const query = new URLSearchParams({ workspace_id: workspaceID });
    return this.request<{ runs: ApiRun[] | null }>(
      `/threads/${encodeURIComponent(threadID)}/runs?${query.toString()}`,
    ).then((data) => (data.runs ?? []).map(normalizeRun));
  }

  public listRunOutputs(runID: string): Promise<ApiRunOutput[]> {
    return this.request<{ outputs: ApiRunOutput[] | null }>(
      `/runs/${encodeURIComponent(runID)}/outputs`,
    ).then((data) => data.outputs ?? []);
  }

  public startRun(runID: string): Promise<ApiRun> {
    return this.mutateRun(runID, 'start');
  }

  public stopRun(runID: string): Promise<ApiRun> {
    return this.mutateRun(runID, 'stop');
  }

  public inputRun(runID: string, text: string): Promise<void> {
    return this.request<void>(`/runs/${encodeURIComponent(runID)}/input`, {
      method: 'POST',
      body: JSON.stringify({ text }),
    });
  }

  public retryRun(runID: string, nextRunID: string): Promise<ApiRun> {
    return this.request<ApiRun>(`/runs/${encodeURIComponent(runID)}/retry`, {
      method: 'POST',
      body: JSON.stringify({ run_id: nextRunID }),
    }).then(normalizeRun);
  }

  private mutateRun(runID: string, action: 'start' | 'stop'): Promise<ApiRun> {
    return this.request<ApiRun>(`/runs/${encodeURIComponent(runID)}/${action}`, {
      method: 'POST',
    }).then(normalizeRun);
  }

  public createMessage(workspaceID: string, threadID: string, body: string): Promise<ApiMessage> {
    return this.request<ApiMessage>(`/threads/${encodeURIComponent(threadID)}/messages`, {
      method: 'POST',
      body: JSON.stringify({ workspace_id: workspaceID, body, actor_type: 'user' }),
    });
  }

  public createLocalAgentMessage(
    workspaceID: string,
    threadID: string,
    agentID: string,
    body: string,
  ): Promise<ApiMessage> {
    return this.request<ApiMessage>(`/threads/${encodeURIComponent(threadID)}/agent-messages`, {
      method: 'POST',
      body: JSON.stringify({ workspace_id: workspaceID, agent_id: agentID, body }),
    });
  }

  public createAgentActivity(
    workspaceID: string,
    threadID: string,
    agentID: string,
    status: 'started' | 'completed' | 'failed' | 'cancelled' | 'waiting_approval',
  ): Promise<ApiMessage> {
    return this.request<ApiMessage>(`/threads/${encodeURIComponent(threadID)}/agent-activity`, {
      method: 'POST',
      body: JSON.stringify({ workspace_id: workspaceID, agent_id: agentID, status }),
    });
  }
}
