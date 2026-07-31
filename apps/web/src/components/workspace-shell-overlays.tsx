import type { useWorkspaceController } from './use-workspace-controller';
import type { Dispatch, SetStateAction } from 'react';
import type { ConnectionDialogMode } from './channel-connection-dialog';
import { ChannelConnectionOverlay } from './channel-context';
import { PublishDialog } from './publish-dialog';
import { TerminalLaunchDialog } from './terminal-launch-dialog';
import { WorkspaceModalLayer } from './workspace-modal-layer';
import type { ToolPanel } from './workspace-main-column';
import type { useTerminalSessions } from './use-terminal-sessions';

type Controller = ReturnType<typeof useWorkspaceController>;

export function WorkspaceShellOverlays({
  controller,
  channelDialogOpen,
  channelSettingsOpen,
  membersOpen,
  channelParentID,
  parentName,
  connectionMode,
  publishOpen,
  terminalRepositoryID,
  terminals,
  setChannelDialogOpen,
  setChannelSettingsOpen,
  setMembersOpen,
  setConnectionMode,
  setPublishOpen,
  setTerminalRepositoryID,
  setToolPanel,
}: {
  controller: Controller;
  channelDialogOpen: boolean;
  channelSettingsOpen: boolean;
  membersOpen: boolean;
  channelParentID: string;
  parentName?: string | undefined;
  connectionMode?: ConnectionDialogMode | undefined;
  publishOpen: boolean;
  terminalRepositoryID?: string | undefined;
  terminals: ReturnType<typeof useTerminalSessions>;
  setChannelDialogOpen: Dispatch<SetStateAction<boolean>>;
  setChannelSettingsOpen: Dispatch<SetStateAction<boolean>>;
  setMembersOpen: Dispatch<SetStateAction<boolean>>;
  setConnectionMode: (mode: ConnectionDialogMode | undefined) => void;
  setPublishOpen: (open: boolean) => void;
  setTerminalRepositoryID: (id: string | undefined) => void;
  setToolPanel: (tool: ToolPanel) => void;
}) {
  const channel = controller.channels.find((item) => item.id === controller.activeChannelID);
  const repository = controller.repositories.find(
    (item) => item.id === controller.selectedRepositoryID,
  );
  return (
    <>
      <WorkspaceModalLayer
        controller={controller}
        channelDialogOpen={channelDialogOpen}
        channelSettingsOpen={channelSettingsOpen}
        membersOpen={membersOpen}
        channelParentID={channelParentID}
        parentName={parentName}
        onChannelDialogOpenChange={setChannelDialogOpen}
        onChannelSettingsOpenChange={setChannelSettingsOpen}
        onMembersOpenChange={setMembersOpen}
      />
      {connectionMode && channel && (
        <ChannelConnectionOverlay
          mode={connectionMode}
          channel={channel}
          controller={controller}
          onClose={() => setConnectionMode(undefined)}
        />
      )}
      {publishOpen && controller.activeWorkspace && repository && controller.activeRun && (
        <PublishDialog
          api={controller.api}
          workspace={controller.activeWorkspace}
          repository={repository}
          run={controller.activeRun}
          onClose={() => setPublishOpen(false)}
        />
      )}
      <TerminalDialogLayer
        repositoryID={terminalRepositoryID}
        controller={controller}
        terminals={terminals}
        setToolPanel={setToolPanel}
        onClose={() => setTerminalRepositoryID(undefined)}
      />
    </>
  );
}

function TerminalDialogLayer({
  repositoryID,
  controller,
  terminals,
  setToolPanel,
  onClose,
}: {
  repositoryID: string | undefined;
  controller: Controller;
  terminals: ReturnType<typeof useTerminalSessions>;
  setToolPanel: (tool: ToolPanel) => void;
  onClose: () => void;
}) {
  const repository = controller.repositoryOptions.find((item) => item.id === repositoryID);
  if (!repository) return null;
  return (
    <TerminalLaunchDialog
      repository={repository}
      onClose={onClose}
      onLaunch={(target, level) => {
        terminals.open(repository, target, level);
        setToolPanel('terminal');
        onClose();
      }}
    />
  );
}
