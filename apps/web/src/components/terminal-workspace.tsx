'use client';

import { useState } from 'react';
import { Plus, Terminal, X } from 'lucide-react';
import type { RepositorySummary } from '@/lib/workspace-state';
import { TerminalPanel } from './terminal-panel';
import type { TerminalSession } from './use-terminal-sessions';

export function TerminalWorkspace({
  sessions,
  activeID,
  repositories,
  selectedRepositoryID,
  onActivate,
  onOpen,
  onClose,
}: {
  sessions: readonly TerminalSession[];
  activeID: string | undefined;
  repositories: readonly RepositorySummary[];
  selectedRepositoryID: string | undefined;
  onActivate: (id: string) => void;
  onOpen: (repository: RepositorySummary | undefined) => void;
  onClose: (id: string) => void;
}) {
  const [repositoryID, setRepositoryID] = useState(
    selectedRepositoryID ?? repositories[0]?.id ?? '',
  );
  return (
    <section className="terminal-workspace" aria-label="Resource terminals">
      <header className="terminal-workspace-header">
        <div className="terminal-tabs" role="tablist" aria-label="Open terminals">
          {sessions.map((session) => (
            <div
              className={`terminal-tab ${session.id === activeID ? 'is-active' : ''}`}
              key={session.id}
            >
              <button
                role="tab"
                aria-selected={session.id === activeID}
                onClick={() => onActivate(session.id)}
              >
                <Terminal size={12} />
                {shortRepositoryName(session.repositoryName)}
              </button>
              <button
                aria-label={`Close ${session.repositoryName} terminal`}
                onClick={() => onClose(session.id)}
              >
                <X size={11} />
              </button>
            </div>
          ))}
        </div>
        <div className="terminal-add">
          <select
            aria-label="Terminal resource"
            value={repositoryID}
            onChange={(event) => setRepositoryID(event.target.value)}
          >
            {repositories.map((repository) => (
              <option value={repository.id} key={repository.id}>
                {repository.fullName}
              </option>
            ))}
          </select>
          <button
            className="icon-button context-secondary-action"
            disabled={!repositoryID}
            aria-label="Open new terminal"
            title="Open new terminal"
            onClick={() =>
              onOpen(repositories.find((repository) => repository.id === repositoryID))
            }
          >
            <Plus size={14} />
          </button>
        </div>
      </header>
      {sessions.map((session) => (
        <div className="terminal-session" hidden={session.id !== activeID} key={session.id}>
          <TerminalPanel url={session.url} tokenSource={session.tokenSource} />
        </div>
      ))}
    </section>
  );
}

function shortRepositoryName(name: string): string {
  return name.split('/').filter(Boolean).at(-1) ?? name;
}
