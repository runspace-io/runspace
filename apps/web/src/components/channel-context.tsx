'use client';

import type { ApiChannel } from '@/lib/api-client';
import type { ChannelSettingsDraft } from './channel-model';
import type { useWorkspaceController } from './use-workspace-controller';
import { ChannelDetailsRail } from './channel-details-rail';
import { ChannelConnectionDialog, type ConnectionDialogMode } from './channel-connection-dialog';
import {
  agentConnectionDraft,
  channelRepositoryIDs,
  repositoryConnectionDraft,
  repositoryConnectionsDraft,
} from './channel-config';
import type { ToolPanel } from './workspace-main-column';
import { ChannelAgentChats, type AgentChatSelection } from './channel-agent-chats';

type Controller = ReturnType<typeof useWorkspaceController>;

export function ChannelContextRail({
  controller,
  channel,
  onClose,
  onOpenSettings,
  onRequestConnection,
  onOpenRepositoryTool,
  onOpenAgentTask,
  chatRevision,
  onOpenAgentChat,
  onChatShared,
}: {
  controller: Controller;
  channel?: ApiChannel | undefined;
  onClose: () => void;
  onOpenSettings: () => void;
  onRequestConnection: (mode: ConnectionDialogMode) => void;
  onOpenRepositoryTool: (repositoryID: string, tool: Exclude<ToolPanel, undefined>) => void;
  onOpenAgentTask: () => void;
  chatRevision: number;
  onOpenAgentChat: (chat: AgentChatSelection) => void;
  onChatShared: (chat: AgentChatSelection) => void;
}) {
  return (
    <ChannelDetailsRail
      channel={channel}
      repositories={controller.repositoryOptions}
      selectedRepositoryID={controller.selectedRepositoryID}
      agentAvailable={controller.agentAvailable}
      agents={controller.agents}
      onClose={onClose}
      onRepositoryChange={controller.setSelectedRepositoryID}
      onOpenAgentTask={onOpenAgentTask}
      onRequestRepositoryConnection={() => onRequestConnection('repository')}
      onRequestAgentConnection={() => onRequestConnection('agent')}
      onOpenSettings={onOpenSettings}
      onOpenRepositoryTool={onOpenRepositoryTool}
      agentChats={
        controller.activeWorkspace && controller.threadID ? (
          <ChannelAgentChats
            api={controller.api}
            workspaceID={controller.activeWorkspace.id}
            threadID={controller.threadID}
            revision={chatRevision}
            onOpen={onOpenAgentChat}
            onShared={onChatShared}
          />
        ) : undefined
      }
    />
  );
}

export function ChannelConnectionOverlay({
  mode,
  channel,
  controller,
  onClose,
}: {
  mode: ConnectionDialogMode;
  channel: ApiChannel;
  controller: Controller;
  onClose: () => void;
}) {
  const connectedIDs = channelRepositoryIDs(channel);
  const available = controller.repositories.filter(
    (repository) => !connectedIDs.includes(repository.id),
  );
  const connectAgent = async (protocol: ChannelSettingsDraft['agentProtocol'], command: string) => {
    return controller.updateChannel(agentConnectionDraft(channel, protocol, command));
  };
  return (
    <ChannelConnectionDialog
      mode={mode}
      api={controller.api}
      channel={channel}
      repositories={available}
      onClose={onClose}
      onConnectRepository={async (repositoryIDs, repositoryURLs) => {
        const attachedIDs = [...repositoryIDs];
        for (const repositoryURL of repositoryURLs.filter((url) => url.startsWith('local:'))) {
          const repository = await controller.mirrorLocalRepository(repositoryURL.slice(6));
          if (!repository) return false;
          attachedIDs.push(repository.id);
        }
        if (attachedIDs.length > 0) {
          return controller.updateChannel(repositoryConnectionsDraft(channel, attachedIDs));
        }
        const repositoryURL = repositoryURLs[0] ?? '';
        return controller.updateChannel(repositoryConnectionDraft(channel, '', repositoryURL));
      }}
      onConnectAgent={connectAgent}
    />
  );
}
