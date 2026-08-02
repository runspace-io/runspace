import type {
  ApiAgentTask,
  ApiAgentTaskMessage,
  ApiMessage,
  ApiTaskGrant,
  ApiTaskQuestion,
} from './api-types';
import { ResourceApiClient } from './resource-api-client';

export class AgentTaskApiClient extends ResourceApiClient {
  // The server-side transcript. Realtime task events carry no bodies, so this
  // is how a viewer reads what the agent said — and it enforces the grant.
  public listAgentTaskMessages(taskID: string): Promise<ApiAgentTaskMessage[]> {
    return this.request<{ messages: ApiAgentTaskMessage[] | null }>(
      `/agent-tasks/${encodeURIComponent(taskID)}/messages`,
    ).then((data) => data.messages ?? []);
  }

  /** canAnswer reflects this viewer's grant, so the UI never offers a control
   * the server would reject. */
  public listTaskQuestions(
    taskID: string,
  ): Promise<{ questions: ApiTaskQuestion[]; canAnswer: boolean }> {
    return this.request<{ questions: ApiTaskQuestion[] | null; can_answer?: boolean }>(
      `/agent-tasks/${encodeURIComponent(taskID)}/questions`,
    ).then((data) => ({
      questions: data.questions ?? [],
      canAnswer: data.can_answer === true,
    }));
  }

  /** Unblocks a waiting agent. An empty optionID cancels the request. */
  public answerTaskQuestion(
    taskID: string,
    questionID: string,
    optionID: string,
  ): Promise<ApiTaskQuestion> {
    return this.request<ApiTaskQuestion>(
      `/agent-tasks/${encodeURIComponent(taskID)}/questions/${encodeURIComponent(questionID)}/answer`,
      { method: 'POST', body: JSON.stringify({ option_id: optionID }) },
    );
  }

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
