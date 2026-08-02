'use client';

import { Bot, Check, Code2, GitBranch, GitCompare, Terminal } from 'lucide-react';
import type { ReactNode } from 'react';
import type { ApiChannel } from '@/lib/api-client';
import type { RepositorySummary } from '@/lib/workspace-state';
import type { useWorkspaceController } from './use-workspace-controller';
import type { ConnectionDialogMode } from './channel-connection-dialog';
import type { ToolPanel } from './workspace-main-column';
import { HeaderPopover } from './header-popover';
import { ContextAddButton } from './channel-connection-forms';
import { agentDescription, ContextEmptyState } from './channel-details-parts';
import { ChannelAgentChats, type AgentChatSelection } from './channel-agent-chats';

type Controller = ReturnType<typeof useWorkspaceController>;

/**
 * Replaces the old docked "channel context" sidebar with two independent
 * popovers anchored in the main header, so the chat column keeps its full
 * width and resource/agent management is one click away instead of always on.
 */
export function ChannelHeaderPopovers({
  controller,
  channel,
  onRequestConnection,
  onOpenRepositoryTool,
  onOpenAgentTask,
  chatRevision,
  onOpenAgentChat,
  onChatShared,
}: {
  controller: Controller;
  channel?: ApiChannel | undefined;
  onRequestConnection: (mode: ConnectionDialogMode) => void;
  onOpenRepositoryTool: (repositoryID: string, tool: Exclude<ToolPanel, undefined>) => void;
  onOpenAgentTask: () => void;
  chatRevision: number;
  onOpenAgentChat: (chat: AgentChatSelection) => void;
  onChatShared: (chat: AgentChatSelection) => void;
}) {
  if (!channel) return null;
  const agent = agentDescription(channel, controller.agents);
  return (
    <>
      <ResourcePopover
        repositories={controller.repositoryOptions}
        selectedRepositoryID={controller.selectedRepositoryID}
        onRepositoryChange={controller.setSelectedRepositoryID}
        onRequestConnection={() => onRequestConnection('repository')}
        onOpenTool={onOpenRepositoryTool}
      />
      <AgentPopover
        label={agent.label}
        detail={agent.detail}
        available={controller.agentAvailable}
        onOpenTask={onOpenAgentTask}
        onRequestConnection={() => onRequestConnection('agent')}
        chats={
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
    </>
  );
}

function ResourcePopover({
  repositories,
  selectedRepositoryID,
  onRepositoryChange,
  onRequestConnection,
  onOpenTool,
}: {
  repositories: readonly RepositorySummary[];
  selectedRepositoryID: string | undefined;
  onRepositoryChange: (id: string) => void;
  onRequestConnection: () => void;
  onOpenTool: (repositoryID: string, tool: Exclude<ToolPanel, undefined>) => void;
}) {
  const active = repositories.find((repository) => repository.id === selectedRepositoryID);
  const triggerLabel = active
    ? active.fullName
    : repositories.length > 0
      ? 'Resources'
      : 'Connect resource';
  return (
    <HeaderPopover
      title="Resources"
      panelLabel="Resources"
      trigger={() => (
        <>
          <GitBranch size={14} />
          <span className="header-popover-trigger-label">{triggerLabel}</span>
        </>
      )}
    >
      <h2 className="header-popover-heading">Resources</h2>
      <p className="header-popover-scope">
        Workspace-owned, not this channel&apos;s — using one here doesn&apos;t take it away from
        other channels.
      </p>
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
    </HeaderPopover>
  );
}

function RepositoryTools({
  repository,
  onOpenTool,
}: {
  repository: RepositorySummary;
  onOpenTool: (repositoryID: string, tool: Exclude<ToolPanel, undefined>) => void;
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

function AgentPopover({
  label,
  detail,
  available,
  onOpenTask,
  onRequestConnection,
  chats,
}: {
  label: string;
  detail: string;
  available: boolean;
  onOpenTask: () => void;
  onRequestConnection: () => void;
  chats?: ReactNode;
}) {
  return (
    <HeaderPopover
      title="Agent"
      panelLabel="Agent"
      trigger={() => (
        <>
          <Bot size={14} />
          <span className="header-popover-trigger-label">{available ? label : 'No agent'}</span>
          {available && <span className="status-dot online" />}
        </>
      )}
    >
      <h2 className="header-popover-heading">Agent</h2>
      <p className="header-popover-scope">
        Yours, not this channel&apos;s — installed on your device, usable in any channel you&apos;re
        in.
      </p>
      <div className="agent-summary">
        <span className="context-option-icon">
          <Bot size={16} />
        </span>
        <span>
          <strong>{label}</strong>
          <small>{detail}</small>
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
    </HeaderPopover>
  );
}
