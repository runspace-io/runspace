import type { ApiAgentTask, ApiMessage, ApiTaskGrant } from './api-types';
import { ResourceApiClient } from './resource-api-client';

export class AgentTaskApiClient extends ResourceApiClient {
  public listTaskGrants(
    workspaceID: string,
    taskID: string,
    agentID: string,
  ): Promise<ApiTaskGrant[]> {
    const query = new URLSearchParams({ workspace_id: workspaceID, agent_id: agentID });
    return this.request<{ grants: ApiTaskGrant[] | null }>(
      `/agent-tasks/${encodeURIComponent(taskID)}/grants?${query.toString()}`,
    ).then((data) => data.grants ?? []);
  }

  public grantTaskAccess(
    workspaceID: string,
    taskID: string,
    agentID: string,
    principalID: string,
    role: ApiTaskGrant['role'],
  ): Promise<ApiTaskGrant> {
    return this.request<ApiTaskGrant>(
      `/agent-tasks/${encodeURIComponent(taskID)}/grants/${encodeURIComponent(principalID)}`,
      {
        method: 'PUT',
        body: JSON.stringify({ workspace_id: workspaceID, agent_id: agentID, role }),
      },
    );
  }

  public upsertAgentTask(task: ApiAgentTask): Promise<ApiAgentTask> {
    return this.request<ApiAgentTask>(`/agent-tasks/${encodeURIComponent(task.id)}`, {
      method: 'PUT',
      body: JSON.stringify(task),
    });
  }

  public listAgentTasks(workspaceID: string, threadID: string): Promise<ApiAgentTask[]> {
    const query = new URLSearchParams({ thread_id: threadID });
    return this.request<{ tasks: ApiAgentTask[] | null }>(
      `/workspaces/${encodeURIComponent(workspaceID)}/agent-tasks?${query.toString()}`,
    ).then((data) => data.tasks ?? []);
  }

  public inputAgentTask(
    taskID: string,
    input: string,
  ): Promise<{ outputs: Array<{ kind: string; text: string }> }> {
    return this.request(`/agent-tasks/${encodeURIComponent(taskID)}/input`, {
      method: 'POST',
      body: JSON.stringify({ input }),
    });
  }

  public cancelAgentTask(taskID: string): Promise<{ status: 'cancelled' }> {
    return this.request(`/agent-tasks/${encodeURIComponent(taskID)}/cancel`, {
      method: 'POST',
    });
  }

  public shareTaskArtifact(taskID: string, body: string): Promise<ApiMessage> {
    return this.request<ApiMessage>(`/agent-tasks/${encodeURIComponent(taskID)}/artifacts`, {
      method: 'POST',
      body: JSON.stringify({ body }),
    });
  }
}
