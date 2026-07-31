'use client';

import { useState, type FormEvent, type ReactNode } from 'react';
import { ExternalLink, GitPullRequest, X } from 'lucide-react';
import { WorkspaceApiClient, type ApiPublishResult, type ApiRun } from '@/lib/api-client';
import type { RepositorySummary, WorkspaceSummary } from '@/lib/workspace-state';
import { useModalFocus } from './use-modal-focus';

export function PublishDialog({
  api,
  workspace,
  repository,
  run,
  onClose,
}: {
  api: WorkspaceApiClient;
  workspace: WorkspaceSummary;
  repository: RepositorySummary;
  run: ApiRun;
  onClose: () => void;
}) {
  const dialogRef = useModalFocus(true, onClose);
  const [branch, setBranch] = useState(defaultBranch(run.id));
  const [commitMessage, setCommitMessage] = useState(run.prompt);
  const [title, setTitle] = useState(run.prompt);
  const [body, setBody] = useState('');
  const [result, setResult] = useState<ApiPublishResult>();
  const [error, setError] = useState<string>();
  const [publishing, setPublishing] = useState(false);
  const publish = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setPublishing(true);
    setError(undefined);
    try {
      setResult(
        await api.publishRun(workspace.id, run.id, {
          repository_id: repository.id,
          branch: branch.trim(),
          base: repository.defaultBranch,
          commit_message: commitMessage.trim(),
          title: title.trim(),
          body: body.trim(),
        }),
      );
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Unable to publish this run.');
    } finally {
      setPublishing(false);
    }
  };
  return (
    <div
      className="dialog-backdrop"
      role="presentation"
      onMouseDown={(event) => event.target === event.currentTarget && onClose()}
    >
      <section
        ref={dialogRef}
        className="workspace-dialog publish-dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby="publish-dialog-title"
      >
        <div className="dialog-header">
          <div>
            <p className="eyebrow">GIT REVIEW</p>
            <h2 id="publish-dialog-title">Publish pull request</h2>
          </div>
          <button className="icon-button" aria-label="Close publish dialog" onClick={onClose}>
            <X size={17} />
          </button>
        </div>
        {result ? (
          <PublishSuccess result={result} />
        ) : (
          <form className="channel-form" onSubmit={publish}>
            <PublishField label="Git repository">
              <input value={repository.fullName} readOnly />
            </PublishField>
            <PublishField label="Base branch">
              <input value={repository.defaultBranch} readOnly />
            </PublishField>
            <PublishField label="New branch">
              <input required value={branch} onChange={(event) => setBranch(event.target.value)} />
            </PublishField>
            <PublishField label="Commit message">
              <input
                required
                value={commitMessage}
                onChange={(event) => setCommitMessage(event.target.value)}
              />
            </PublishField>
            <PublishField label="Pull request title">
              <input required value={title} onChange={(event) => setTitle(event.target.value)} />
            </PublishField>
            <PublishField label="Description">
              <textarea value={body} onChange={(event) => setBody(event.target.value)} />
            </PublishField>
            {error && <p className="dialog-error">{error}</p>}
            <div className="dialog-actions">
              <button type="button" className="dialog-secondary" onClick={onClose}>
                Cancel
              </button>
              <button type="submit" className="dialog-primary" disabled={publishing}>
                <GitPullRequest size={15} />
                {publishing ? 'Publishing…' : 'Create pull request'}
              </button>
            </div>
          </form>
        )}
      </section>
    </div>
  );
}

function PublishField({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="channel-field">
      <span>{label}</span>
      {children}
    </label>
  );
}

function PublishSuccess({ result }: { result: ApiPublishResult }) {
  return (
    <div className="publish-success">
      <GitPullRequest size={24} />
      <h3>Pull request #{result.pull_request.number} created</h3>
      <p>
        Branch <code>{result.branch.name}</code> was committed and pushed successfully.
      </p>
      <a href={result.pull_request.url} target="_blank" rel="noreferrer">
        Open pull request <ExternalLink size={14} />
      </a>
    </div>
  );
}

function defaultBranch(runID: string): string {
  return `runspace/${runID.replace(/[^a-zA-Z0-9._-]/g, '-').slice(0, 48)}`;
}
