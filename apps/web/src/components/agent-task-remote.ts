import type { Dispatch, SetStateAction } from 'react';
import type { ApiAgentTask, WorkspaceApiClient } from '@/lib/api-client';
import { getLocalAgentSession, type LocalAgentSession } from '@/lib/host-agent-client';
import type { AgentTaskProps } from './agent-task-controller';

export async function loadChannelTask(request: {
  api: WorkspaceApiClient;
  workspaceID: string;
  threadID: string;
  agentID: string;
  resourceID: string;
  taskID: string;
  registered: boolean;
}) {
  const { api, workspaceID, threadID, agentID, resourceID, taskID, registered } = request;
  const tasks = registered ? await api.listAgentTasks(workspaceID, threadID) : [];
  const existing = tasks.find(
    (task) => task.id === taskID && task.agent_id === agentID && task.resource_id === resourceID,
  );
  if (existing && existing.owner_id !== api.actorID) {
    return {
      session: remoteSession(existing),
      remoteTask: existing,
      title: existing.title,
    };
  }
  const session = await getLocalAgentSession({
    userID: api.actorID,
    agentID,
    resourceID,
    threadID,
    taskID: existing?.id ?? taskID,
  });
  return {
    session,
    remoteTask: undefined,
    title: existing?.title ?? session.title,
  };
}

export async function runRemoteTask(
  props: AgentTaskProps,
  task: ApiAgentTask,
  prompt: string,
  state: {
    setSession: Dispatch<SetStateAction<LocalAgentSession | undefined>>;
    setError: (message: string | undefined) => void;
    setBusy: (busy: boolean) => void;
  },
) {
  const { setSession, setError, setBusy } = state;
  try {
    const response = await props.api.inputAgentTask(task.id, prompt);
    const now = new Date().toISOString();
    setSession((current) =>
      current
        ? {
            ...current,
            status: 'completed',
            messages: [
              ...current.messages,
              ...response.outputs.map((output, index) => ({
                id: `remote_output_${Date.now()}_${index}`,
                role: 'agent' as const,
                body: output.text,
                created_at: now,
              })),
            ],
          }
        : current,
    );
  } catch (reason) {
    setError(taskError(reason, 'The owner Host Agent could not complete this instruction.'));
    setSession((current) => (current ? { ...current, status: 'failed' } : current));
  } finally {
    setBusy(false);
  }
}

export function saveTaskMetadata(
  props: AgentTaskProps,
  taskID: string,
  resourceID: string,
  title: string,
  status: ApiAgentTask['status'],
) {
  const now = new Date().toISOString();
  return props.api
    .upsertAgentTask({
      id: taskID,
      workspace_id: props.workspaceID,
      thread_id: props.threadID,
      owner_id: props.api.actorID,
      agent_id: props.agentID,
      resource_id: resourceID,
      title,
      status,
      created_at: now,
      updated_at: now,
    })
    .then((task) => {
      props.onTaskChange?.(task);
      return task;
    });
}

export function titleFromWork(input: string): string {
  const firstLine = input
    .trim()
    .split(/\r?\n/, 1)[0]
    ?.replace(/^(please|can you|could you)\s+/i, '')
    .replace(/[.!?]+$/, '')
    .trim();
  if (!firstLine) return 'Untitled agent task';
  return firstLine.length > 72 ? `${firstLine.slice(0, 69).trimEnd()}…` : firstLine;
}

function remoteSession(task: ApiAgentTask): LocalAgentSession {
  return {
    id: task.id,
    title: task.title,
    agent_id: task.agent_id,
    resource_id: task.resource_id,
    thread_id: task.thread_id,
    status: task.status,
    pause_support: 'cancel-only',
    messages: [],
    updated_at: task.updated_at,
  };
}

function taskError(reason: unknown, fallback: string) {
  return reason instanceof Error && reason.message ? reason.message : fallback;
}
