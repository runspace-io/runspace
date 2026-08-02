import { useEffect, useRef, type Dispatch, type SetStateAction } from 'react';
import {
  eventToTimelineItem,
  WorkspaceApiClient,
  type ApiAgentInstallation,
  type ApiChannel,
  type ApiEvent,
  type ApiRun,
} from '@/lib/api-client';
import { agentTaskEvent, eventRunContext } from '@/lib/api-normalizers';
import {
  ReconnectingRealtimeSocket,
  type RealtimeFrame,
  type RealtimeStatus,
} from '@/lib/realtime-client';
import type { RepositorySummary, TimelineItem, WorkspaceSummary } from '@/lib/workspace-state';
import { updateActiveRun } from './run-sync';

export type WorkspaceSyncState = {
  setWorkspaces: (items: WorkspaceSummary[]) => void;
  setActiveWorkspace: Dispatch<SetStateAction<WorkspaceSummary | undefined>>;
  setRepositories: (items: RepositorySummary[]) => void;
  setRealtimeStatus: (status: RealtimeStatus) => void;
  setTimeline: Dispatch<SetStateAction<TimelineItem[]>>;
  setThreadID: (id: string | undefined) => void;
  setChannels: Dispatch<SetStateAction<ApiChannel[]>>;
  setAgents: Dispatch<SetStateAction<ApiAgentInstallation[]>>;
  setActiveChannelID: (id: string | undefined) => void;
  setActiveRun: Dispatch<SetStateAction<ApiRun | undefined>>;
  onAgentTaskEvent?: (update: {
    taskID: string;
    status: string | undefined;
    questionID: string | undefined;
  }) => void;
};

export function useWorkspaceSync(
  api: WorkspaceApiClient,
  context: WorkspaceSyncContext,
  state: WorkspaceSyncState,
): void {
  const { workspaceID } = context;
  useWorkspaceList(api, state);
  useWorkspaceData(api, workspaceID, state);
  useRealtime(api, context, state);
}

export type WorkspaceSyncContext = {
  workspaceID: string | undefined;
  activeThreadID: string | undefined;
  activeChannelID: string | undefined;
  activeRunID: string | undefined;
  agents: readonly ApiAgentInstallation[];
};

function useWorkspaceList(api: WorkspaceApiClient, state: WorkspaceSyncState) {
  useEffect(() => {
    let active = true;
    void api
      .listWorkspaces()
      .then((items) => {
        if (!active) return;
        state.setWorkspaces(items);
        state.setActiveWorkspace((current) => initialWorkspace(current, items));
      })
      .catch(() => undefined);
    return () => {
      active = false;
    };
  }, [api, state]);
}

export function initialWorkspace(
  current: WorkspaceSummary | undefined,
  items: readonly WorkspaceSummary[],
): WorkspaceSummary | undefined {
  return current ?? items[0];
}

function useWorkspaceData(
  api: WorkspaceApiClient,
  workspaceID: string | undefined,
  state: WorkspaceSyncState,
) {
  useEffect(() => {
    let active = true;
    resetWorkspaceState(state);
    if (!workspaceID) return () => void (active = false);
    void api
      .listChannels(workspaceID)
      .then((items) => active && state.setChannels(items))
      .catch(() => active && state.setChannels([]));
    void api
      .listResources(workspaceID)
      .then((items) => active && state.setRepositories(items))
      .catch(() => active && state.setRepositories([]));
    void api
      .listWorkspaceAgents(workspaceID)
      .then((items) => active && state.setAgents(items))
      .catch(() => active && state.setAgents([]));
    return () => {
      active = false;
    };
  }, [api, state, workspaceID]);
}

function resetWorkspaceState(state: WorkspaceSyncState) {
  state.setThreadID(undefined);
  state.setActiveChannelID(undefined);
  state.setTimeline([]);
  state.setActiveRun(undefined);
  state.setChannels([]);
  state.setAgents([]);
  state.setRepositories([]);
}

function useRealtime(
  api: WorkspaceApiClient,
  context: WorkspaceSyncContext,
  state: WorkspaceSyncState,
) {
  const { workspaceID } = context;
  const contextRef = useRef(context);
  contextRef.current = context;
  useEffect(() => {
    if (!workspaceID) {
      state.setRealtimeStatus('idle');
      return;
    }
    const client = new ReconnectingRealtimeSocket({
      url: api.realtimeURL(workspaceID),
      workspaceID,
      userID: api.actorID,
      onStatus: state.setRealtimeStatus,
      onFrame: (frame) => {
        const active = contextRef.current;
        handleFrame(frame, { ...active, actorID: api.actorID }, state);
      },
    });
    client.start();
    return () => client.stop();
  }, [api, state, workspaceID]);
}

function handleFrame(
  frame: RealtimeFrame,
  context: WorkspaceSyncContext & { actorID: string },
  state: WorkspaceSyncState,
) {
  const event = frame.event as ApiEvent | undefined;
  if (!event) return;
  const taskUpdate = agentTaskEvent(event);
  if (taskUpdate) {
    state.onAgentTaskEvent?.(taskUpdate);
    return;
  }
  const eventContext = eventRunContext(event);
  if (event.type === 'message.created' && eventContext.threadID !== context.activeThreadID) return;
  if (
    event.type === 'agent.output' &&
    !matchesRun(eventContext, context.activeRunID, context.activeChannelID)
  )
    return;
  const item = eventToTimelineItem(event, context.actorID, context.agents);
  if (item) {
    state.setTimeline((current) =>
      current.some((entry) => entry.id === item.id) ? current : [...current, item],
    );
  }
  if (eventContext.status)
    updateActiveRun(
      state.setActiveRun,
      eventContext.status,
      eventContext.runID,
      context.activeRunID,
    );
}

function matchesRun(
  context: ReturnType<typeof eventRunContext>,
  activeRunID: string | undefined,
  activeChannelID: string | undefined,
) {
  if (activeRunID) return context.runID === activeRunID;
  return Boolean(context.channelID && context.channelID === activeChannelID);
}
