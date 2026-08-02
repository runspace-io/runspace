'use client';

import type { ApiChannel } from '@/lib/api-client';
import type { ChannelSettingsDraft } from './channel-model';
import type { useWorkspaceController } from './use-workspace-controller';
import { ChannelConnectionDialog, type ConnectionDialogMode } from './channel-connection-dialog';
import {
  agentConnectionDraft,
  channelRepositoryIDs,
  repositoryConnectionDraft,
  repositoryConnectionsDraft,
} from './channel-config';

type Controller = ReturnType<typeof useWorkspaceController>;

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
