import type { ApiChannel } from '@/lib/api-client';
import type { useWorkspaceController } from './use-workspace-controller';
import { LeftRail } from './workspace-rails';
import type { ChannelWorkItem } from './use-channel-shared-work';

type Controller = ReturnType<typeof useWorkspaceController>;

export function WorkspaceNavigation({
  controller,
  channelWork,
  activeWorkID,
  onCreateChannel,
  onOpenChannel,
  onOpenWork,
  onOpenResourceCenter,
  onOpenFile,
}: {
  controller: Controller;
  channelWork: readonly ChannelWorkItem[];
  activeWorkID?: string | undefined;
  onCreateChannel: (parentID: string) => void;
  onOpenChannel: (channel: ApiChannel) => void;
  onOpenWork: (item: ChannelWorkItem) => void;
  onOpenResourceCenter: () => void;
  onOpenFile: (path: string) => void;
}) {
  return (
    <LeftRail
      open={controller.leftOpen}
      tree={controller.tree}
      expandedDirectories={controller.expandedDirectories}
      selectedFilePath={controller.selected?.path}
      channels={controller.channels}
      activeChannelID={controller.activeChannelID}
      channelWork={channelWork}
      activeWorkID={activeWorkID}
      onClose={() => controller.setLeftOpen(false)}
      onRequestCreateChannel={(parentID = '') => onCreateChannel(parentID)}
      onOpenChannel={onOpenChannel}
      onOpenWork={onOpenWork}
      onOpenResourceCenter={onOpenResourceCenter}
      onSelectFile={onOpenFile}
      onToggleDirectory={controller.toggleDirectory}
    />
  );
}
