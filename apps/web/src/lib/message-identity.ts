import type { ApiAgentInstallation } from './api-types';

export type MessageIdentity = {
  author: string;
  provider?: string;
  providerID?: string;
};

export function resolveMessageIdentity(
  actorID: string,
  actorType: string,
  viewerID: string | undefined,
  agents: readonly ApiAgentInstallation[] = [],
): MessageIdentity {
  if (actorType === 'user') {
    return { author: actorID === viewerID ? 'You' : displayName(actorID) };
  }
  const agent = agents.find((item) => item.id === actorID);
  if (agent) {
    return {
      author: displayName(agent.owner_id),
      provider: agent.name,
      providerID: agent.registry_id,
    };
  }
  if (actorType === 'agent') return { author: displayName(actorID), provider: 'Agent' };
  return { author: displayName(actorID) };
}

export function displayName(id: string): string {
  const value = id
    .trim()
    .replace(/^@/, '')
    .replace(/[-_.]+/g, ' ');
  if (!value) return 'Unknown';
  if (value.startsWith('local agent ')) return 'Agent';
  return value.replace(/\b\p{L}/gu, (letter) => letter.toLocaleUpperCase());
}
