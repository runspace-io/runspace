'use client';

import { Bot, GitBranch } from 'lucide-react';
import type { ApiChannel, WorkspaceApiClient } from '@/lib/api-client';
import type { RepositorySummary } from '@/lib/workspace-state';
import type { ChannelSettingsDraft } from './channel-model';
import { RepositoryConnectionForm } from './channel-connection-forms';
import { AgentConnectionForm } from './agent-connection-form';
import { channelAgentCommand } from './channel-config';
import { ConnectionDialog } from './connection-dialog';

export type ConnectionDialogMode = 'repository' | 'agent';

type ChannelConnectionDialogProps = {
  mode: ConnectionDialogMode;
  api: WorkspaceApiClient;
  channel: ApiChannel;
  repositories: readonly RepositorySummary[];
  onClose: () => void;
  onConnectRepository: (
    repositoryIDs: readonly string[],
    repositoryURLs: readonly string[],
  ) => Promise<boolean>;
  onConnectAgent: (
    protocol: ChannelSettingsDraft['agentProtocol'],
    command: string,
  ) => Promise<boolean>;
};

export function ChannelConnectionDialog(props: ChannelConnectionDialogProps) {
  const copy = connectionCopy(props.mode, props.channel.name);
  return (
    <ConnectionDialog {...copy} onClose={props.onClose}>
      <ConnectionDialogContent {...props} />
    </ConnectionDialog>
  );
}

function ConnectionDialogContent({
  mode,
  api,
  channel,
  repositories,
  onClose,
  onConnectRepository,
  onConnectAgent,
}: ChannelConnectionDialogProps) {
  if (mode === 'repository') {
    return (
      <RepositoryConnectionForm
        available={repositories}
        onConnect={onConnectRepository}
        onDone={onClose}
      />
    );
  }
  return (
    <AgentConnectionForm
      api={api}
      initialCommand={channelAgentCommand(channel)}
      onConnect={onConnectAgent}
      onConnected={onClose}
    />
  );
}

function connectionCopy(mode: ConnectionDialogMode, channelName: string) {
  if (mode === 'repository') {
    return {
      eyebrow: 'CHANNEL SOURCE',
      title: 'Connect resource',
      description: `Attach a workspace resource, remote Git source, or local folder to #${channelName}.`,
      icon: <GitBranch size={18} />,
    };
  }
  return {
    eyebrow: 'ACTIVE COLLABORATOR',
    title: 'Connect agent',
    description: `Choose the agent runtime that collaborates actively in #${channelName}.`,
    icon: <Bot size={18} />,
  };
}
