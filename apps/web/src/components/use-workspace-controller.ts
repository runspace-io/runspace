import { useMemo, useState, type Dispatch, type SetStateAction } from 'react';
import { WorkspaceApiClient, type ApiAgentInstallation, type ApiChannel } from '@/lib/api-client';
import type { RealtimeStatus } from '@/lib/realtime-client';
import {
  type RepositorySummary,
  type TimelineItem,
  type WorkspaceSummary,
} from '@/lib/workspace-state';
import type { DialogMode } from './workspace-dialog';
import { useChannelComposer } from './use-channel-composer';
import { createWorkspaceControllerActions } from './workspace-controller-actions';
import { useWorkspaceSync, type WorkspaceSyncState } from './workspace-sync';
import { useRepositoryTree } from './use-repository-tree';
import { channelRepositoryIDs } from './channel-config';
import { connectLocalMirror } from '@/lib/host-agent-client';
export function useWorkspaceController(userID: string) {
  const api = useMemo(() => new WorkspaceApiClient({ userID }), [userID]);
  const [timeline, setTimeline] = useState<TimelineItem[]>([]);
  const [draft, setDraft] = useState('');
  const [leftOpen, setLeftOpen] = useState(false);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [dialogMode, setDialogMode] = useState<DialogMode>('list');
  const [workspaceName, setWorkspaceName] = useState('');
  const [formError, setFormError] = useState<string | undefined>();
  const [workspaces, setWorkspaces] = useState<WorkspaceSummary[]>([]);
  const [activeWorkspace, setActiveWorkspace] = useState<WorkspaceSummary | undefined>();
  const [repositories, setRepositories] = useState<RepositorySummary[]>([]);
  const [channels, setChannels] = useState<ApiChannel[]>([]);
  const [agents, setAgents] = useState<ApiAgentInstallation[]>([]);
  const [activeChannelID, setActiveChannelID] = useState<string>();
  const [selectedRepositoryID, setSelectedRepositoryID] = useState<string>();
  const [realtimeStatus, setRealtimeStatus] = useState<RealtimeStatus>('idle');
  const [threadID, setThreadID] = useState<string>();
  const [activeRun, setActiveRun] = useState<import('@/lib/api-client').ApiRun>();
  const activeChannel = channels.find((channel) => channel.id === activeChannelID);
  const repositoryIDs = channelRepositoryIDs(activeChannel);
  const repositoryID = selectedChannelRepository(repositoryIDs, selectedRepositoryID);
  const selectedRepository = repositories.find((repository) => repository.id === repositoryID);
  const composer = useChannelComposer({
    api,
    workspace: activeWorkspace,
    channel: activeChannel,
    threadID,
    draft,
    setDraft,
    setTimeline,
    setError: setFormError,
  });
  const syncState = useMemo<WorkspaceSyncState>(
    () => ({
      setWorkspaces,
      setActiveWorkspace,
      setRepositories,
      setRealtimeStatus,
      setTimeline,
      setThreadID,
      setChannels,
      setAgents,
      setActiveChannelID,
      setActiveRun,
    }),
    [],
  );
  const syncContext = {
    workspaceID: activeWorkspace?.id,
    activeThreadID: threadID,
    activeChannelID,
    activeRunID: activeRun?.id,
    agents,
  };
  useWorkspaceSync(api, syncContext, syncState);
  const repositoryTree = useRepositoryTree({
    api,
    workspaceID: activeWorkspace?.id,
    repositoryID,
    repositoryProvider: selectedRepository?.provider,
    setError: setFormError,
  });
  const actions = createWorkspaceControllerActions({
    api,
    workspace: activeWorkspace,
    workspaceName,
    repositories,
    activeChannel,
    state: {
      setWorkspaces,
      setActiveWorkspace,
      setRepositories,
      setChannels,
      setThreadID,
      setActiveChannelID,
      setTimeline,
      setActiveRun,
      setSelectedRepositoryID,
      setWorkspaceName,
      setDialogOpen,
      setDialogMode,
      setFormError,
    },
  });
  return {
    api,
    timeline,
    draft,
    setDraft,
    leftOpen,
    setLeftOpen,
    dialogOpen,
    setDialogOpen,
    dialogMode,
    setDialogMode,
    workspaceName,
    setWorkspaceName,
    formError,
    setFormError,
    workspaces,
    activeWorkspace,
    setActiveWorkspace,
    repositories,
    agents,
    channels,
    activeChannelID,
    tree: repositoryTree.tree,
    expandedDirectories: repositoryTree.expandedDirectories,
    repositoryReady: repositoryTree.ready,
    terminalURL: repositoryTree.terminalURL,
    realtimeStatus,
    threadID,
    selected: repositoryTree.selectedFile,
    selectFile: repositoryTree.selectFile,
    toggleDirectory: repositoryTree.toggleDirectory,
    send: composer.send,
    agentAvailable: composer.agentAvailable,
    activeRun,
    createWorkspace: actions.createWorkspace,
    createChannel: actions.createChannel,
    updateChannel: actions.updateChannel,
    openChannel: actions.openChannel,
    selectedRepositoryID: repositoryID,
    repositoryOptions: repositories.filter((repository) => repositoryIDs.includes(repository.id)),
    setSelectedRepositoryID,
    openDialog: actions.openDialog,
    mirrorLocalRepository: (path: string) =>
      mirrorLocalRepository(path, userID, activeWorkspace, setRepositories, setFormError),
  };
}

async function mirrorLocalRepository(
  path: string,
  userID: string,
  workspace: WorkspaceSummary | undefined,
  setRepositories: Dispatch<SetStateAction<RepositorySummary[]>>,
  setError: Dispatch<SetStateAction<string | undefined>>,
): Promise<RepositorySummary | undefined> {
  if (!workspace) {
    setError('Select a workspace before connecting a local resource.');
    return undefined;
  }
  setError(undefined);
  try {
    const repository = await connectLocalMirror({ path, userID, workspaceID: workspace.id });
    setRepositories((current) => [
      ...current.filter((item) => item.id !== repository.id),
      repository,
    ]);
    return repository;
  } catch (error) {
    setError(error instanceof Error ? error.message : 'Could not connect local resource.');
    return undefined;
  }
}

function selectedChannelRepository(
  repositoryIDs: readonly string[],
  selectedID: string | undefined,
): string | undefined {
  if (selectedID && repositoryIDs.includes(selectedID)) return selectedID;
  return repositoryIDs[0];
}
