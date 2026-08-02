'use client';

import { Bot, Braces, RefreshCw, Sparkles, SquareTerminal, X, type LucideIcon } from 'lucide-react';
import { useEffect, useState } from 'react';
import type { ApiAgentInstallation, WorkspaceApiClient } from '@/lib/api-client';
import { displayName } from '@/lib/message-identity';
import { useLocalAgentDiscovery } from './use-local-agent-discovery';

/**
 * The workspace-wide counterpart to Resource Center: agent installations are
 * owned by a person, not a channel, and usable anywhere they're a member —
 * this is the one place that visibility is actually shown.
 */
export function AgentCenter({
  api,
  workspaceID,
  onClose,
}: {
  api: WorkspaceApiClient;
  workspaceID: string;
  onClose: () => void;
}) {
  const [agents, setAgents] = useState<ApiAgentInstallation[]>([]);
  const [loading, setLoading] = useState(true);
  const [revision, setRevision] = useState(0);
  const discovery = useLocalAgentDiscovery(api, '');

  useEffect(() => {
    let active = true;
    setLoading(true);
    void api
      .listWorkspaceAgents(workspaceID)
      .then((items) => active && setAgents(items))
      .catch(() => active && setAgents([]))
      .finally(() => active && setLoading(false));
    return () => {
      active = false;
    };
  }, [api, workspaceID, revision]);

  const scan = async () => {
    await discovery.discover();
    setRevision((value) => value + 1);
  };

  return (
    <section className="resource-center agent-center" aria-labelledby="agent-center-title">
      <header className="resource-center-header">
        <span className="resource-center-mark">
          <Bot size={18} />
        </span>
        <div>
          <p className="eyebrow">Your agent runtimes</p>
          <h2 id="agent-center-title">Agent Center</h2>
          <p>
            Installed on a device, not a channel — usable in any channel of any workspace you belong
            to.
          </p>
        </div>
        <div className="resource-center-actions">
          <button
            className="secondary-button"
            onClick={() => void scan()}
            disabled={discovery.loading}
          >
            <RefreshCw size={14} className={discovery.loading ? 'is-spinning' : ''} />
            Scan this device
          </button>
          <button className="icon-button" onClick={onClose} aria-label="Close Agent Center">
            <X size={16} />
          </button>
        </div>
      </header>
      {discovery.error && <p className="connection-field-error">{discovery.error}</p>}
      <div className="resource-center-grid">
        {agents.map((agent) => (
          <AgentCard key={agent.id} agent={agent} viewerID={api.actorID} />
        ))}
        {!loading && agents.length === 0 ? (
          <div className="resource-center-empty">
            <Bot size={20} />
            <strong>No agents in this workspace yet</strong>
            <span>Scan this device to register any agent runtimes you have installed.</span>
          </div>
        ) : null}
        {loading ? <p className="graph-loading">Reading workspace agents…</p> : null}
      </div>
    </section>
  );
}

function AgentCard({ agent, viewerID }: { agent: ApiAgentInstallation; viewerID: string }) {
  const Icon = agentIcon(agent.registry_id);
  const owner = agent.owner_id === viewerID ? 'You' : displayName(agent.owner_id);
  const capabilities = agent.capabilities ?? [];
  return (
    <div className="resource-card">
      <span className="resource-card-icon" data-kind="agent">
        <Icon size={16} />
      </span>
      <span className="resource-card-copy">
        <span>
          <strong>{agent.name}</strong>
          <small>{agent.status}</small>
        </span>
        <p>
          {owner}-owned · Host Agent{agent.description ? ` · ${agent.description}` : ''}
        </p>
        <small>
          {capabilities.length > 0 ? capabilities.join(', ') : 'No capabilities listed'}
        </small>
      </span>
    </div>
  );
}

function agentIcon(registryID: string): LucideIcon {
  const id = registryID.toLocaleLowerCase();
  if (id.includes('codex')) return SquareTerminal;
  if (id.includes('claude')) return Sparkles;
  if (id.includes('opencode')) return Braces;
  return Bot;
}
