import type { ApiChannel } from '@/lib/api-client';
import type { useWorkspaceController } from './use-workspace-controller';
import type { ConnectionDialogMode } from './channel-connection-dialog';
import { ChannelContextRail } from './channel-context';
import type { AgentChatSelection } from './channel-agent-chats';
import type { ToolPanel } from './workspace-main-column';

type Controller = ReturnType<typeof useWorkspaceController>;

export function WorkspaceChannelDetails({
  controller,
  channel,
  chatRevision,
  onClose,
  onOpenSettings,
  onRequestConnection,
  onOpenNewChat,
  onOpenChat,
  onChatShared,
  onOpenRepositoryTool,
}: {
  controller: Controller;
  channel?: ApiChannel | undefined;
  chatRevision: number;
  onClose: () => void;
  onOpenSettings: () => void;
  onRequestConnection: (mode: ConnectionDialogMode) => void;
  onOpenNewChat: () => void;
  onOpenChat: (chat: AgentChatSelection) => void;
  onChatShared: (chat: AgentChatSelection) => void;
  onOpenRepositoryTool: (repositoryID: string, tool: Exclude<ToolPanel, undefined>) => void;
}) {
  if (!channel) return null;
  return (
    <ChannelContextRail
      controller={controller}
      channel={channel}
      onClose={onClose}
      onOpenSettings={onOpenSettings}
      onRequestConnection={onRequestConnection}
      onOpenAgentTask={onOpenNewChat}
      chatRevision={chatRevision}
      onOpenAgentChat={onOpenChat}
      onChatShared={onChatShared}
      onOpenRepositoryTool={onOpenRepositoryTool}
    />
  );
}
