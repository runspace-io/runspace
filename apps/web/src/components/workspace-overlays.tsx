import type { WorkspaceApiClient, ApiChannel } from '@/lib/api-client';
import type { RepositorySummary, WorkspaceSummary } from '@/lib/workspace-state';
import { WorkspaceDialog, type DialogMode } from './workspace-dialog';
import { ChannelDialog } from './channel-dialog';
import { ChannelSettingsDialog } from './channel-settings-dialog';
import { MembersDialog } from './members-dialog';
import type { ChannelDraft, ChannelSettingsDraft } from './channel-model';

export function WorkspaceOverlays({
  api,
  error,
  onDismissError,
  workspaceDialog,
  channelDialog,
  channelSettings,
  members,
}: {
  api: WorkspaceApiClient;
  error?: string | undefined;
  onDismissError: () => void;
  workspaceDialog: {
    open: boolean;
    mode: DialogMode;
    workspaces: readonly WorkspaceSummary[];
    activeWorkspace: WorkspaceSummary | undefined;
    workspaceName: string;
    onClose: () => void;
    onModeChange: (mode: DialogMode) => void;
    onWorkspaceNameChange: (name: string) => void;
    onSelectWorkspace: (workspace: WorkspaceSummary) => void;
    onCreateWorkspace: (name?: string) => void;
  };
  channelDialog: {
    open: boolean;
    parentID: string;
    parentName?: string | undefined;
    repositories: readonly RepositorySummary[];
    onClose: () => void;
    onCreate: (draft: ChannelDraft) => Promise<boolean>;
  };
  channelSettings: {
    open: boolean;
    channel?: ApiChannel | undefined;
    onClose: () => void;
    onUpdate: (draft: ChannelSettingsDraft) => Promise<boolean>;
  };
  members: {
    open: boolean;
    workspace?: WorkspaceSummary | undefined;
    onClose: () => void;
  };
}) {
  return (
    <>
      {workspaceDialog.open && <WorkspaceDialog {...workspaceDialog} error={error} />}
      {channelDialog.open && <ChannelDialog {...channelDialog} />}
      {channelSettings.open && channelSettings.channel && (
        <ChannelSettingsDialog
          api={api}
          channel={channelSettings.channel}
          onClose={channelSettings.onClose}
          onUpdate={channelSettings.onUpdate}
        />
      )}
      {members.open && members.workspace && (
        <MembersDialog api={api} workspace={members.workspace} onClose={members.onClose} />
      )}
      {error && !workspaceDialog.open && (
        <button className="form-error" type="button" onClick={onDismissError}>
          {error}
        </button>
      )}
    </>
  );
}
