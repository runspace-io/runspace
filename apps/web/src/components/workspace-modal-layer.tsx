import type { Dispatch, SetStateAction } from 'react';
import type { useWorkspaceController } from './use-workspace-controller';
import { WorkspaceOverlays } from './workspace-overlays';

type Controller = ReturnType<typeof useWorkspaceController>;

export function WorkspaceModalLayer({
  controller,
  channelDialogOpen,
  channelSettingsOpen,
  membersOpen,
  channelParentID,
  parentName,
  onChannelDialogOpenChange,
  onChannelSettingsOpenChange,
  onMembersOpenChange,
}: {
  controller: Controller;
  channelDialogOpen: boolean;
  channelSettingsOpen: boolean;
  membersOpen: boolean;
  channelParentID: string;
  parentName?: string | undefined;
  onChannelDialogOpenChange: Dispatch<SetStateAction<boolean>>;
  onChannelSettingsOpenChange: Dispatch<SetStateAction<boolean>>;
  onMembersOpenChange: Dispatch<SetStateAction<boolean>>;
}) {
  const activeChannel = controller.channels.find(
    (channel) => channel.id === controller.activeChannelID,
  );
  return (
    <WorkspaceOverlays
      api={controller.api}
      error={controller.formError}
      onDismissError={() => controller.setFormError(undefined)}
      workspaceDialog={{
        open: controller.dialogOpen,
        mode: controller.dialogMode,
        workspaces: controller.workspaces,
        activeWorkspace: controller.activeWorkspace,
        workspaceName: controller.workspaceName,
        onClose: () => controller.setDialogOpen(false),
        onModeChange: (mode) => {
          controller.setDialogMode(mode);
          controller.setFormError(undefined);
        },
        onWorkspaceNameChange: controller.setWorkspaceName,
        onSelectWorkspace: (workspace) => {
          controller.setActiveWorkspace(workspace);
          controller.setDialogOpen(false);
        },
        onCreateWorkspace: controller.createWorkspace,
      }}
      channelDialog={{
        open: channelDialogOpen,
        parentID: channelParentID,
        parentName,
        repositories: controller.repositories,
        onClose: () => onChannelDialogOpenChange(false),
        onCreate: controller.createChannel,
      }}
      channelSettings={{
        open: channelSettingsOpen,
        channel: activeChannel,
        onClose: () => onChannelSettingsOpenChange(false),
        onUpdate: controller.updateChannel,
      }}
      members={{
        open: membersOpen,
        workspace: controller.activeWorkspace,
        onClose: () => onMembersOpenChange(false),
      }}
    />
  );
}
