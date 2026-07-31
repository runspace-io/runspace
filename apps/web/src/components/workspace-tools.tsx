import type { ComponentProps } from 'react';
import type { WorkspaceApiClient } from '@/lib/api-client';
import type { RepositorySummary } from '@/lib/workspace-state';
import { ChangeReview } from './change-review';
import { CodeViewer } from './code-viewer';
import { TerminalWorkspace } from './terminal-workspace';
import type { TerminalSession } from './use-terminal-sessions';
import type { ToolPanel } from './workspace-main-column';

export function WorkspaceTools({
  panel,
  file,
  api,
  workspaceID,
  repositoryID,
  terminalSessions,
  activeTerminalID,
  terminalRepositories,
  onTerminalActivate,
  onTerminalOpen,
  onTerminalClose,
}: {
  panel: ToolPanel;
  file: ComponentProps<typeof CodeViewer>['file'];
  api: WorkspaceApiClient;
  workspaceID: string | undefined;
  repositoryID: string | undefined;
  terminalSessions: readonly TerminalSession[];
  activeTerminalID: string | undefined;
  terminalRepositories: readonly RepositorySummary[];
  onTerminalActivate: (id: string | undefined) => void;
  onTerminalOpen: (repository: RepositorySummary | undefined) => void;
  onTerminalClose: (id: string) => void;
}) {
  if (panel === 'code') return <CodeViewer file={file} />;
  if (panel === 'changes' && workspaceID && repositoryID) {
    return <ChangeReview api={api} workspaceID={workspaceID} repositoryID={repositoryID} />;
  }
  if (panel === 'terminal') {
    return (
      <TerminalWorkspace
        sessions={terminalSessions}
        activeID={activeTerminalID}
        repositories={terminalRepositories}
        selectedRepositoryID={repositoryID}
        onActivate={onTerminalActivate}
        onOpen={onTerminalOpen}
        onClose={onTerminalClose}
      />
    );
  }
  return null;
}
