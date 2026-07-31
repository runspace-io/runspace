import type { ApiChannel } from '@/lib/api-client';
import type { ChannelSettingsDraft } from './channel-model';

export function channelRepositoryIDs(channel: ApiChannel | undefined): string[] {
  if (!channel) return [];
  return channel.repository_ids ?? (channel.repository_id ? [channel.repository_id] : []);
}

export function channelAgentProtocol(channel: ApiChannel): ChannelSettingsDraft['agentProtocol'] {
  const agent = channel.config?.agent;
  if (!isRecord(agent)) return 'none';
  if (agent.protocol === 'mock') return 'mock';
  return agent.protocol === 'acp' || typeof agent.command === 'string' ? 'acp' : 'none';
}

export function channelAgentCommand(channel: ApiChannel): string {
  const agent = channel.config?.agent;
  if (!isRecord(agent)) return '';
  if (typeof agent.installation_id === 'string') return agent.installation_id;
  return typeof agent.command === 'string' ? agent.command : '';
}

export function channelSettingsDraft(
  channel: ApiChannel,
  overrides: Partial<ChannelSettingsDraft> = {},
): ChannelSettingsDraft {
  const repositoryIDs = channelRepositoryIDs(channel);
  return {
    name: channel.name,
    repositoryID: repositoryIDs[0] ?? '',
    repositoryIDs,
    repositoryURL: '',
    agentProtocol: channelAgentProtocol(channel),
    agentCommand: channelAgentCommand(channel),
    ...overrides,
  };
}

export function repositoryConnectionDraft(
  channel: ApiChannel,
  repositoryID: string,
  repositoryURL: string,
): ChannelSettingsDraft {
  const repositoryIDs = [
    ...new Set([...channelRepositoryIDs(channel), ...(repositoryID ? [repositoryID] : [])]),
  ];
  return channelSettingsDraft(channel, {
    repositoryID: repositoryIDs[0] ?? '',
    repositoryIDs,
    repositoryURL,
  });
}

export function repositoryConnectionsDraft(
  channel: ApiChannel,
  repositoryIDsToAdd: readonly string[],
): ChannelSettingsDraft {
  const repositoryIDs = [
    ...new Set([...channelRepositoryIDs(channel), ...repositoryIDsToAdd.filter(Boolean)]),
  ];
  return channelSettingsDraft(channel, {
    repositoryID: repositoryIDs[0] ?? '',
    repositoryIDs,
    repositoryURL: '',
  });
}

export function agentConnectionDraft(
  channel: ApiChannel,
  agentProtocol: ChannelSettingsDraft['agentProtocol'],
  agentCommand: string,
): ChannelSettingsDraft {
  return channelSettingsDraft(channel, { agentProtocol, agentCommand });
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === 'object';
}
