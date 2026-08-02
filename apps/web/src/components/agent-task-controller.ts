'use client';

import { useEffect, useMemo, useState } from 'react';
import type { ApiAgentTask, WorkspaceApiClient } from '@/lib/api-client';
import {
  cancelLocalAgentSession,
  getLocalAgentSession,
  promptLocalAgent,
  type LocalAgentSession,
  type LocalTaskMessage,
} from '@/lib/host-agent-client';
import type { RepositorySummary } from '@/lib/workspace-state';
import {
  loadChannelTask,
  runRemoteTask,
  saveTaskMetadata,
  titleFromWork,
} from './agent-task-remote';

export type AgentTaskProps = {
  api: WorkspaceApiClient;
  workspaceID: string;
  threadID: string;
  agentID: string;
  resources: readonly RepositorySummary[];
  initialResourceID?: string | undefined;
  taskID?: string | undefined;
  registered?: boolean | undefined;
  /** Bumped by realtime task events so a viewer reloads someone else's chat. */
  taskRevision?: number | undefined;
  onClose: () => void;
  onTaskChange?: ((task: ApiAgentTask) => void) | undefined;
  onChatChange?: (() => void) | undefined;
};

export function useAgentTask(props: AgentTaskProps) {
  const [resourceID, setResourceID] = useState(
    props.initialResourceID ?? props.resources[0]?.id ?? '',
  );
  const [taskID] = useState(() => props.taskID ?? nextTaskID());
  const [session, setSession] = useState<LocalAgentSession>();
  const [instruction, setInstruction] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const [shared, setShared] = useState<Set<string>>(new Set());
  const [accessOpen, setAccessOpen] = useState(false);
  const [remoteTask, setRemoteTask] = useState<ApiAgentTask>();
  const [title, setTitle] = useState('');
  const [reloadKey, setReloadKey] = useState(0);
  const selectedResource = useMemo(
    () => props.resources.find((resource) => resource.id === resourceID),
    [props.resources, resourceID],
  );
  // Realtime task events and local answers both reload through the same key.
  const revision = reloadKey + (props.taskRevision ?? 0);
  useTaskLoader(props, resourceID, taskID, revision, {
    setSession,
    setError,
    setRemoteTask,
    setTitle,
  });

  const run = async () => {
    const prompt = instruction.trim();
    if (!canRun(prompt, resourceID, busy)) return;
    setBusy(true);
    setInstruction('');
    setError(undefined);
    setSession((current) => optimisticSession(current, props, resourceID, taskID, prompt));
    if (remoteTask) {
      await runRemoteTask(props, remoteTask, prompt, { setSession, setError, setBusy });
      return;
    }
    const nextTitle = title || titleFromWork(prompt);
    setTitle(nextTitle);
    const activeTaskID = session?.id ?? taskID;
    await persistRegisteredChat(props, activeTaskID, resourceID, nextTitle, 'running');
    await publishRegisteredActivity(props, 'started');
    try {
      await promptLocalAgent({
        userID: props.api.actorID,
        agentID: props.agentID,
        resourceID,
        threadID: props.threadID,
        taskID: activeTaskID,
        prompt,
      });
      const completed = await loadSession(props, resourceID, activeTaskID);
      setSession(completed);
      const completedTitle = completed.title || nextTitle;
      setTitle(completedTitle);
      props.onChatChange?.();
      await persistRegisteredChat(props, activeTaskID, resourceID, completedTitle, 'completed');
      await publishRegisteredActivity(props, 'completed');
    } catch (reason) {
      setError(errorMessage(reason, 'The local agent could not complete this instruction.'));
      setSession((current) => (current ? { ...current, status: 'failed' } : current));
      await persistRegisteredChat(props, activeTaskID, resourceID, nextTitle, 'failed');
      await publishRegisteredActivity(props, 'failed');
    } finally {
      setBusy(false);
    }
  };

  const cancel = async () => {
    try {
      if (remoteTask) {
        await props.api.cancelAgentTask(remoteTask.id);
        setSession((current) => (current ? { ...current, status: 'cancelled' } : current));
        return;
      }
      await cancelLocalAgentSession({
        userID: props.api.actorID,
        agentID: props.agentID,
        resourceID,
        threadID: props.threadID,
        taskID,
      });
      setSession((current) => (current ? { ...current, status: 'cancelled' } : current));
      await publishRegisteredActivity(props, 'cancelled');
    } catch (reason) {
      setError(errorMessage(reason, 'The task could not be cancelled.'));
    }
  };

  const share = async (message: LocalTaskMessage) => {
    try {
      if (remoteTask) await props.api.shareTaskArtifact(remoteTask.id, message.body);
      else
        await props.api.createLocalAgentMessage(
          props.workspaceID,
          props.threadID,
          props.agentID,
          message.body,
        );
      setShared((current) => new Set(current).add(message.id));
    } catch (reason) {
      setError(errorMessage(reason, 'The artifact could not be shared.'));
    }
  };

  return {
    resourceID,
    setResourceID,
    session,
    instruction,
    setInstruction,
    busy,
    error,
    shared,
    accessOpen,
    setAccessOpen,
    selectedResource,
    remote: Boolean(remoteTask),
    title,
    run,
    cancel,
    share,
    refresh: () => setReloadKey((current) => current + 1),
  };
}

function useTaskLoader(
  props: AgentTaskProps,
  resourceID: string,
  taskID: string,
  revision: number,
  state: {
    setSession: (session: LocalAgentSession | undefined) => void;
    setError: (message: string | undefined) => void;
    setRemoteTask: (task: ApiAgentTask | undefined) => void;
    setTitle: (title: string) => void;
  },
) {
  const { api, agentID, threadID, workspaceID } = props;
  const { setSession, setError, setRemoteTask, setTitle } = state;
  useEffect(() => {
    let active = true;
    setError(undefined);
    if (!resourceID) {
      setSession(undefined);
      setRemoteTask(undefined);
      return;
    }
    void loadChannelTask({
      api,
      workspaceID,
      threadID,
      agentID,
      resourceID,
      taskID,
      registered: props.registered !== false,
    })
      .then(({ session, remoteTask, title }) => {
        if (!active) return;
        setSession(session);
        setRemoteTask(remoteTask);
        setTitle(title);
      })
      .catch(
        (reason) => active && setError(errorMessage(reason, 'Could not open the channel task.')),
      );
    return () => {
      active = false;
    };
  }, [
    api,
    agentID,
    resourceID,
    setError,
    setRemoteTask,
    setSession,
    setTitle,
    threadID,
    taskID,
    workspaceID,
    revision,
    props.registered,
  ]);
}

function loadSession(props: AgentTaskProps, resourceID: string, taskID: string) {
  return getLocalAgentSession({
    userID: props.api.actorID,
    agentID: props.agentID,
    resourceID,
    threadID: props.threadID,
    taskID,
  });
}

async function publishActivity(
  props: AgentTaskProps,
  status: 'started' | 'completed' | 'failed' | 'cancelled',
) {
  try {
    await props.api.createAgentActivity(props.workspaceID, props.threadID, props.agentID, status);
  } catch {
    // Host execution is authoritative; presence projection can recover independently.
  }
}

function persistRegisteredChat(
  props: AgentTaskProps,
  taskID: string,
  resourceID: string,
  title: string,
  status: ApiAgentTask['status'],
) {
  if (props.registered === false) return Promise.resolve();
  return saveTaskMetadata(props, taskID, resourceID, title, status).then(() => undefined);
}

function publishRegisteredActivity(
  props: AgentTaskProps,
  status: 'started' | 'completed' | 'failed' | 'cancelled',
) {
  return props.registered === false ? Promise.resolve() : publishActivity(props, status);
}

function optimisticSession(
  current: LocalAgentSession | undefined,
  props: AgentTaskProps,
  resourceID: string,
  taskID: string,
  prompt: string,
): LocalAgentSession {
  const now = new Date().toISOString();
  return {
    id: current?.id ?? taskID,
    title: current?.title ?? titleFromWork(prompt),
    agent_id: props.agentID,
    resource_id: resourceID,
    thread_id: props.threadID,
    status: 'running',
    pause_support: current?.pause_support ?? 'cancel-only',
    messages: [
      ...(current?.messages ?? []),
      { id: `optimistic_${Date.now()}`, role: 'user', body: prompt, created_at: now },
    ],
    updated_at: now,
  };
}

export function errorMessage(reason: unknown, fallback: string) {
  return reason instanceof Error && reason.message ? reason.message : fallback;
}

function nextTaskID() {
  return `local_session_${globalThis.crypto?.randomUUID?.() ?? Date.now().toString(36)}`;
}

function canRun(prompt: string, resourceID: string, busy: boolean) {
  return Boolean(prompt && resourceID && !busy);
}
