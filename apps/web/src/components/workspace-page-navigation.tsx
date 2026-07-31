import type { ApiGraphNode } from '@/lib/api-client';
import type { useWorkspaceController } from './use-workspace-controller';
import type { AgentChatSelection } from './channel-agent-chats';
import { selectedWorkID, type ChannelWorkItem } from './use-channel-shared-work';
import { WorkspaceNavigation } from './workspace-navigation';
import type { ToolPanel } from './workspace-main-column';

type Controller = ReturnType<typeof useWorkspaceController>;

export function workspacePageNavigation(input: {
  controller: Controller;
  channelWork: readonly ChannelWorkItem[];
  agentChat: AgentChatSelection | undefined;
  graphNode: ApiGraphNode | undefined;
  setAgentChat: (value: AgentChatSelection | undefined) => void;
  setGraphNode: (value: ApiGraphNode | undefined) => void;
  setChannelParentID: (value: string) => void;
  setChannelDialogOpen: (value: boolean) => void;
  setResourceCenterOpen: (value: boolean) => void;
  setToolPanel: (value: ToolPanel) => void;
}) {
  const { controller, channelWork, agentChat, graphNode } = input;
  return (
    <WorkspaceNavigation
      controller={controller}
      channelWork={channelWork}
      activeWorkID={selectedWorkID(graphNode, agentChat)}
      onCreateChannel={(parentID) => {
        input.setChannelParentID(parentID);
        input.setChannelDialogOpen(true);
      }}
      onOpenChannel={(channel) => {
        input.setResourceCenterOpen(false);
        input.setAgentChat(undefined);
        input.setGraphNode(undefined);
        controller.openChannel(channel);
      }}
      onOpenWork={(item) => {
        input.setResourceCenterOpen(false);
        input.setAgentChat(item.chat);
        input.setGraphNode(item.chat ? undefined : item.node);
      }}
      onOpenResourceCenter={() => {
        input.setAgentChat(undefined);
        input.setGraphNode(undefined);
        input.setResourceCenterOpen(true);
      }}
      onOpenFile={(path) => {
        controller.selectFile(path);
        input.setToolPanel('code');
      }}
    />
  );
}
