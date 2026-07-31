import {
  Bot,
  Check,
  Code2,
  GitBranch,
  GitCompare,
  PanelRightClose,
  Settings2,
  Terminal,
} from 'lucide-react';
import type { ReactNode } from 'react';
import type { ApiAgentInstallation, ApiChannel } from '@/lib/api-client';
import type { RepositorySummary } from '@/lib/workspace-state';
import { ContextAddButton } from './channel-connection-forms';
import { agentDescription, ContextEmptyState, ContextHeading } from './channel-details-parts';
import type { ToolPanel } from './workspace-main-column';

type ChannelDetailsRailProps = {
  open: boolean;
  channel?: ApiChannel | undefined;
  repositories: readonly RepositorySummary[];
  selectedRepositoryID?: string | undefined;
  agentAvailable: boolean;
  agents: readonly ApiAgentInstallation[];
  onClose: () => void;
  onRepositoryChange: (id: string) => void;
  onOpenAgentTask: () => void;
  agentChats?: ReactNode;
  onRequestRepositoryConnection: () => void;
  onRequestAgentConnection: () => void;
  onOpenSettings: () => void;
  onOpenRepositoryTool: (repositoryID: string, tool: Exclude<ToolPanel, undefined>) => void;
};

export function ChannelDetailsRail({
  open,
  channel,
  repositories,
  selectedRepositoryID,
  agentAvailable,
  agents,
  onClose,
  onRepositoryChange,
  onOpenAgentTask,
  agentChats,
  onRequestRepositoryConnection,
  onRequestAgentConnection,
  onOpenSettings,
  onOpenRepositoryTool,
}: ChannelDetailsRailProps) {
  if (!channel) return null;

  return (
    <aside className={`channel-details-rail ${open ? 'is-open' : ''}`} aria-label="Channel context">
      <header className="details-rail-header">
        <div>
          <span className="details-kicker">Channel context</span>
          <strong>#{channel.name}</strong>
        </div>
        <button
          className="icon-button details-close-button"
          aria-label="Close channel context"
          onClick={onClose}
        >
          <PanelRightClose size={16} />
        </button>
      </header>

      <RepositoryContext
        repositories={repositories}
        selectedRepositoryID={selectedRepositoryID}
        onRepositoryChange={onRepositoryChange}
        onRequestConnection={onRequestRepositoryConnection}
        onOpenTool={onOpenRepositoryTool}
      />
      <AgentContext
        channel={channel}
        available={agentAvailable}
        agents={agents}
        onOpenTask={onOpenAgentTask}
        onRequestConnection={onRequestAgentConnection}
        chats={agentChats}
      />

      <button className="details-settings-button" onClick={onOpenSettings}>
        <Settings2 size={15} />
        <span>
          <strong>Channel settings</strong>
          <small>Name, secrets, and administration</small>
        </span>
      </button>
    </aside>
  );
}

function RepositoryContext({
  repositories,
  selectedRepositoryID,
  onRepositoryChange,
  onRequestConnection,
  onOpenTool,
}: Pick<ChannelDetailsRailProps, 'repositories' | 'selectedRepositoryID' | 'onRepositoryChange'> & {
  onRequestConnection: () => void;
  onOpenTool: ChannelDetailsRailProps['onOpenRepositoryTool'];
}) {
  return (
    <section className="context-section" aria-label="Channel resources">
      <ContextHeading index="01" title="Resources" value={String(repositories.length)} />
      {repositories.length > 0 ? (
        <div className="context-option-list">
          {repositories.map((repository) => {
            const selected = repository.id === selectedRepositoryID;
            return (
              <div className="repository-context-item" key={repository.id}>
                <button
                  className={`context-option ${selected ? 'is-selected' : ''}`}
                  aria-pressed={selected}
                  onClick={() => onRepositoryChange(repository.id)}
                >
                  <span className="context-option-icon">
                    <GitBranch size={15} />
                  </span>
                  <span className="context-option-copy">
                    <strong>{repository.fullName}</strong>
                    <small>
                      {repository.provider === 'folder'
                        ? 'Local folder'
                        : repository.provider === 'mirror'
                          ? `Local Git · ${repository.defaultBranch}`
                          : repository.defaultBranch}
                    </small>
                  </span>
                  {selected && <Check size={14} aria-label="Selected" />}
                </button>
                {selected && <RepositoryTools repository={repository} onOpenTool={onOpenTool} />}
              </div>
            );
          })}
        </div>
      ) : (
        <ContextEmptyState
          title="No resource connected"
          body="Connect a resource to browse files, open a terminal, and publish changes."
        />
      )}
      <ContextAddButton label="Connect resource" onClick={onRequestConnection} />
    </section>
  );
}

function RepositoryTools({
  repository,
  onOpenTool,
}: {
  repository: RepositorySummary;
  onOpenTool: ChannelDetailsRailProps['onOpenRepositoryTool'];
}) {
  return (
    <div className="repository-tools" aria-label={`${repository.fullName} tools`}>
      <button onClick={() => onOpenTool(repository.id, 'code')} aria-label="Open code" title="Code">
        <Code2 size={14} />
        <span className="sr-only">Code</span>
      </button>
      {repository.provider !== 'folder' && (
        <button
          onClick={() => onOpenTool(repository.id, 'changes')}
          aria-label="Review changes"
          title="Changes"
        >
          <GitCompare size={14} />
          <span className="sr-only">Changes</span>
        </button>
      )}
      <button
        onClick={() => onOpenTool(repository.id, 'terminal')}
        aria-label="Open terminal"
        title="Terminal"
      >
        <Terminal size={14} />
        <span className="sr-only">Terminal</span>
      </button>
    </div>
  );
}

function AgentContext({
  channel,
  available,
  onOpenTask,
  onRequestConnection,
  chats,
  agents,
}: {
  channel: ApiChannel;
  available: boolean;
  onOpenTask: () => void;
  onRequestConnection: () => void;
  chats?: ReactNode;
  agents: readonly ApiAgentInstallation[];
}) {
  const agent = agentDescription(channel, agents);
  return (
    <section className="context-section" aria-labelledby="channel-agent-title">
      <ContextHeading
        index="02"
        title="Agent"
        value={available ? 'Ready' : 'Off'}
        ready={available}
      />
      <div className="agent-summary">
        <span className="context-option-icon">
          <Bot size={16} />
        </span>
        <span>
          <strong>{agent.label}</strong>
          <small>{agent.detail}</small>
        </span>
      </div>
      <div className="context-actions">
        <button className="context-primary-action" disabled={!available} onClick={onOpenTask}>
          <Bot size={14} />
          New agent chat
        </button>
      </div>
      {chats}
      <ContextAddButton
        label={available ? 'Change agent connection' : 'Connect agent'}
        onClick={onRequestConnection}
      />
    </section>
  );
}
