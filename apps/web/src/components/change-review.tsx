'use client';

import { DiffEditor } from '@monaco-editor/react';
import { RefreshCw } from 'lucide-react';
import type { ApiRepositoryChange, WorkspaceApiClient } from '@/lib/api-client';
import { languageForPath } from './code-viewer';
import { useChangeReview } from './use-change-review';

export function ChangeReview({
  api,
  workspaceID,
  repositoryID,
}: {
  api: WorkspaceApiClient;
  workspaceID: string;
  repositoryID: string;
}) {
  const review = useChangeReview(api, workspaceID, repositoryID);
  return (
    <section className="change-review" aria-label="Resource changes">
      <header className="change-review-header">
        <span>{review.changes.length} changed files</span>
        <button
          type="button"
          className="tiny-button"
          aria-label="Refresh resource changes"
          onClick={() => void review.refresh()}
        >
          <RefreshCw size={13} />
        </button>
      </header>
      {review.error && (
        <p className="change-review-state is-error" role="alert">
          {review.error}
        </p>
      )}
      {review.error && review.changes.length === 0 ? null : review.loading ? (
        <p className="change-review-state">Reading Git status…</p>
      ) : review.changes.length === 0 ? (
        <p className="change-review-state">Working tree is clean.</p>
      ) : (
        <div className="change-review-body">
          <ChangeList
            changes={review.changes}
            selectedPath={review.selectedPath}
            onSelect={review.setSelectedPath}
          />
          <div className="change-diff">
            {review.diff ? (
              <DiffEditor
                height="100%"
                original={review.diff.original}
                modified={review.diff.modified}
                language={languageForPath(review.diff.path)}
                theme="vs-dark"
                options={{
                  readOnly: true,
                  renderSideBySide: true,
                  minimap: { enabled: false },
                  originalEditable: false,
                }}
              />
            ) : (
              <p className="change-review-state">Loading diff…</p>
            )}
          </div>
        </div>
      )}
    </section>
  );
}

function ChangeList({
  changes,
  selectedPath,
  onSelect,
}: {
  changes: readonly ApiRepositoryChange[];
  selectedPath: string | undefined;
  onSelect: (path: string) => void;
}) {
  return (
    <div className="change-list" aria-label="Changed files">
      {changes.map((change) => (
        <button
          type="button"
          key={change.path}
          className={selectedPath === change.path ? 'is-selected' : ''}
          aria-pressed={selectedPath === change.path}
          onClick={() => onSelect(change.path)}
        >
          <span className={`change-status is-${change.status}`}>{statusLabel(change.status)}</span>
          <span>{change.path}</span>
        </button>
      ))}
    </div>
  );
}

function statusLabel(status: ApiRepositoryChange['status']): string {
  return {
    added: 'A',
    modified: 'M',
    deleted: 'D',
    renamed: 'R',
    untracked: 'U',
  }[status];
}
