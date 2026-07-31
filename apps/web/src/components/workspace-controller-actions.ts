import type { Dispatch, SetStateAction } from 'react';
import type { ApiChannel, ApiRun, WorkspaceApiClient } from '@/lib/api-client';
import type { RepositorySummary, TimelineItem, WorkspaceSummary } from '@/lib/workspace-state';
import type { ChannelDraft, ChannelSettingsDraft } from './channel-model';
import { createChannelRequest, openChannelRequest, updateChannelRequest } from './channel-actions';
import type { DialogMode } from './workspace-dialog';
import { createWorkspaceRequest } from './workspace-actions';

type ActionState = {
  setWorkspaces: Dispatch<SetStateAction<WorkspaceSummary[]>>;
  setActiveWorkspace: Dispatch<SetStateAction<WorkspaceSummary | undefined>>;
  setRepositories: Dispatch<SetStateAction<RepositorySummary[]>>;
  setChannels: Dispatch<SetStateAction<ApiChannel[]>>;
  setThreadID: (id: string | undefined) => void;
  setActiveChannelID: (id: string | undefined) => void;
  setTimeline: Dispatch<SetStateAction<TimelineItem[]>>;
  setActiveRun: Dispatch<SetStateAction<ApiRun | undefined>>;
  setSelectedRepositoryID: (id: string | undefined) => void;
  setWorkspaceName: (name: string) => void;
  setDialogOpen: (open: boolean) => void;
  setDialogMode: (mode: DialogMode) => void;
  setFormError: (error: string | undefined) => void;
};

export function createWorkspaceControllerActions(input: {
  api: WorkspaceApiClient;
  workspace: WorkspaceSummary | undefined;
  workspaceName: string;
  repositories: readonly RepositorySummary[];
  activeChannel: ApiChannel | undefined;
  state: ActionState;
}) {
  const { api, workspace, workspaceName, repositories, activeChannel, state } = input;
  const channelState = {
    setChannels: state.setChannels,
    setRepositories: state.setRepositories,
    setThreadID: state.setThreadID,
    setActiveChannelID: state.setActiveChannelID,
    setTimeline: state.setTimeline,
    setFormError: state.setFormError,
  };
  return {
    openDialog: (mode: DialogMode) => openWorkspaceDialog(mode, state),
    createWorkspace: (name = workspaceName) =>
      createWorkspaceRequest({
        api,
        name,
        setWorkspaces: state.setWorkspaces,
        setActiveWorkspace: state.setActiveWorkspace,
        setWorkspaceName: state.setWorkspaceName,
        setDialogOpen: state.setDialogOpen,
        setError: state.setFormError,
      }),
    createChannel: (draft: ChannelDraft) =>
      createChannelRequest({ api, workspace, repositories, draft, state: channelState }),
    updateChannel: (draft: ChannelSettingsDraft) =>
      updateActiveChannel({
        api,
        workspace,
        repositories,
        channel: activeChannel,
        draft,
        state: channelState,
      }),
    openChannel: (channel: ApiChannel) => {
      state.setSelectedRepositoryID(firstRepositoryID(channel));
      openChannelRequest({
        api,
        workspace,
        channel,
        setActiveChannelID: state.setActiveChannelID,
        setThreadID: state.setThreadID,
        setTimeline: state.setTimeline,
        setActiveRun: state.setActiveRun,
        setFormError: state.setFormError,
      });
    },
  };
}

function updateActiveChannel(input: {
  api: WorkspaceApiClient;
  workspace: WorkspaceSummary | undefined;
  repositories: readonly RepositorySummary[];
  channel: ApiChannel | undefined;
  draft: ChannelSettingsDraft;
  state: Parameters<typeof updateChannelRequest>[0]['state'];
}) {
  const { api, workspace, repositories, channel, draft, state } = input;
  if (!channel) return Promise.resolve(false);
  return updateChannelRequest({ api, workspace, channel, repositories, draft, state });
}

function openWorkspaceDialog(mode: DialogMode, state: ActionState) {
  state.setDialogMode(mode);
  state.setFormError(undefined);
  state.setDialogOpen(true);
}

function firstRepositoryID(channel: ApiChannel): string | undefined {
  return channel.repository_ids?.[0] ?? channel.repository_id;
}
