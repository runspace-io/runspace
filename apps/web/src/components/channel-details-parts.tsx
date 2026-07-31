import { Circle } from 'lucide-react';
import type { ApiAgentInstallation, ApiChannel } from '@/lib/api-client';
import { displayName } from '@/lib/message-identity';
import { isRecord } from './channel-model';

export function ContextHeading({
  index,
  title,
  value,
  ready = false,
}: {
  index: string;
  title: string;
  value: string;
  ready?: boolean;
}) {
  return (
    <div className="context-section-heading">
      <div>
        <span className="context-index">{index}</span>
        <h2>{title}</h2>
      </div>
      <span className={`context-status ${ready ? 'is-ready' : ''}`}>
        {ready && <Circle size={7} fill="currentColor" />}
        {value}
      </span>
    </div>
  );
}

export function ContextEmptyState({ title, body }: { title: string; body: string }) {
  return (
    <div className="context-empty-state">
      <strong>{title}</strong>
      <p>{body}</p>
    </div>
  );
}

export function agentDescription(
  channel: ApiChannel,
  agents: readonly ApiAgentInstallation[] = [],
): { label: string; detail: string } {
  const value = channel.config?.agent;
  if (!isRecord(value)) {
    return channel.parent_id
      ? { label: 'Inherited agent', detail: 'Uses the parent channel runtime' }
      : { label: 'No agent configured', detail: 'Choose a runtime in channel settings' };
  }
  if (value.protocol === 'mock') {
    return { label: 'Built-in agent', detail: 'Development runtime' };
  }
  if (value.protocol === 'acp') return acpDescription(value, agents);
  return { label: 'Configured agent', detail: 'Channel runtime' };
}

function acpDescription(value: Record<string, unknown>, agents: readonly ApiAgentInstallation[]) {
  const installationID = typeof value.installation_id === 'string' ? value.installation_id : '';
  const installation = agents.find((item) => item.id === installationID);
  if (installation) {
    return {
      label: installation.name,
      detail: `${displayName(installation.owner_id)}-owned · Host Agent · ${installation.status}`,
    };
  }
  return {
    label: 'ACP agent',
    detail:
      typeof value.installation_id === 'string'
        ? 'User-owned Host Agent'
        : typeof value.command === 'string'
          ? value.command
          : 'External runtime',
  };
}
