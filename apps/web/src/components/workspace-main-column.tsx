import type { ComponentProps, ReactNode } from 'react';
import { GitBranch, GitPullRequest, PanelRightOpen, Settings2 } from 'lucide-react';
import { Timeline } from './timeline';
import { ChannelComposer } from './channel-composer';
import type { ApiGraphNode, WorkspaceApiClient } from '@/lib/api-client';
import type { RepositorySummary } from '@/lib/workspace-state';
import type { TerminalSession } from './use-terminal-sessions';
import { WorkspaceTools } from './workspace-tools';

export type ToolPanel = 'code' | 'changes' | 'terminal' | undefined;

export function WorkspaceMainColumn({
  workspaceName,
  workspaceID,
  channelName,
  repositoryName,
  repositoryBranch,
  repositoryGit,
  repositoryID,
  chatReady,
  timeline,
  draft,
  file,
  toolPanel,
  runAvailable,
  onDraftChange,
  onSend,
  onRunAgent,
  onOpenGraphNode,
  onOpenChannelSettings,
  onOpenChannelDetails,
  onOpenPublish,
  terminalState,
  terminalRepositories,
  onTerminalOpen,
  publishAvailable,
  api,
  agentChat,
}: {
  workspaceName: string | undefined;
  workspaceID: string | undefined;
  channelName: string | undefined;
  repositoryName: string | undefined;
  repositoryBranch: string | undefined;
  repositoryGit: boolean;
  repositoryID: string | undefined;
  chatReady: boolean;
  timeline: ComponentProps<typeof Timeline>['items'];
  draft: string;
  file: ComponentProps<typeof WorkspaceTools>['file'];
  toolPanel: ToolPanel;
  runAvailable: boolean;
  onDraftChange: (value: string) => void;
  onSend: () => void;
  onRunAgent: () => void;
  onOpenGraphNode: (node: ApiGraphNode) => void;
  onOpenChannelSettings: () => void;
  onOpenChannelDetails: () => void;
  onOpenPublish: () => void;
  terminalState: {
    sessions: readonly TerminalSession[];
    activeID: string | undefined;
    setActiveID: (id: string | undefined) => void;
    close: (id: string) => void;
  };
  terminalRepositories: readonly RepositorySummary[];
  onTerminalOpen: (repository: RepositorySummary | undefined) => void;
  publishAvailable: boolean;
  api: WorkspaceApiClient;
  agentChat?: ReactNode;
}) {
  return (
    <section className="main-column">
      <div className="main-header">
        <div>
          <p className="eyebrow">
            {channelName ? `CHANNEL / ${channelName}` : 'WORKSPACE OVERVIEW'}
          </p>
          <h1>{channelName ?? workspaceName ?? 'Your workspace'}</h1>
          <RepositoryIdentity name={repositoryName} branch={repositoryBranch} git={repositoryGit} />
        </div>
        <HeaderActions
          channelAvailable={Boolean(channelName)}
          onOpenChannelSettings={onOpenChannelSettings}
          onOpenChannelDetails={onOpenChannelDetails}
          onOpenPublish={onOpenPublish}
          publishAvailable={publishAvailable}
        />
      </div>
      {channelName ? (
        agentChat ? (
          agentChat
        ) : (
          <>
            <Timeline
              items={timeline}
              api={api}
              workspaceID={workspaceID ?? ''}
              onOpenNode={onOpenGraphNode}
            />
            <WorkspaceTools
              panel={toolPanel}
              file={file}
              api={api}
              workspaceID={workspaceID}
              repositoryID={repositoryID}
              terminalSessions={terminalState.sessions}
              activeTerminalID={terminalState.activeID}
              terminalRepositories={terminalRepositories}
              onTerminalActivate={terminalState.setActiveID}
              onTerminalOpen={onTerminalOpen}
              onTerminalClose={terminalState.close}
            />
            {chatReady ? (
              <ChannelComposer
                api={api}
                workspaceID={workspaceID ?? ''}
                draft={draft}
                runAvailable={runAvailable}
                onDraftChange={onDraftChange}
                onSend={onSend}
                onRunAgent={onRunAgent}
              />
            ) : (
              <p className="chat-loading">Opening channel conversation…</p>
            )}
          </>
        )
      ) : (
        <ChannelEmptyState />
      )}
    </section>
  );
}

function ChannelEmptyState() {
  return (
    <section className="channel-empty-state">
      <span className="eyebrow">COLLABORATION STARTS IN A CHANNEL</span>
      <h2>Select or create a channel</h2>
      <p>Channels keep chat, resources, agents, and shared configuration together.</p>
    </section>
  );
}

function RepositoryIdentity({
  name,
  branch,
  git,
}: {
  name: string | undefined;
  branch: string | undefined;
  git: boolean;
}) {
  if (!name) return null;
  return (
    <div className="active-repository">
      <span>{name}</span>
      {git && (
        <span className="active-repository-branch">
          <GitBranch size={12} />
          {branch}
        </span>
      )}
    </div>
  );
}

function HeaderActions({
  channelAvailable,
  onOpenChannelSettings,
  onOpenChannelDetails,
  onOpenPublish,
  publishAvailable,
}: {
  channelAvailable: boolean;
  onOpenChannelSettings: () => void;
  onOpenChannelDetails: () => void;
  onOpenPublish: () => void;
  publishAvailable: boolean;
}) {
  return (
    <div className="main-header-actions">
      {channelAvailable && (
        <button
          className="icon-button mobile-context-button"
          onClick={onOpenChannelDetails}
          aria-label="Open channel context"
          title="Channel context"
        >
          <PanelRightOpen size={16} />
        </button>
      )}
      {channelAvailable && (
        <button
          className="icon-button"
          onClick={onOpenChannelSettings}
          aria-label="Channel settings"
          title="Channel settings"
        >
          <Settings2 size={16} />
        </button>
      )}
      {publishAvailable && (
        <button
          className="icon-button publish-button"
          onClick={onOpenPublish}
          aria-label="Publish changes"
          title="Publish changes"
        >
          <GitPullRequest size={16} />
        </button>
      )}
    </div>
  );
}
