export function channelLocalAgentID(
  config: Record<string, unknown> | undefined,
): string | undefined {
  const agent = config?.agent;
  if (!agent || typeof agent !== 'object' || !('installation_id' in agent)) return undefined;
  return typeof agent.installation_id === 'string' ? agent.installation_id : undefined;
}

export function channelAgentID(
  channel: { config?: Record<string, unknown> } | undefined,
): string | undefined {
  return channelLocalAgentID(channel?.config);
}

export function channelHasAgent(config: Record<string, unknown> | undefined): boolean {
  const agent = config?.agent;
  if (!agent || typeof agent !== 'object') return false;
  if ('protocol' in agent && agent.protocol === 'mock') return true;
  return Boolean(
    ('command' in agent && agent.command) || ('installation_id' in agent && agent.installation_id),
  );
}
