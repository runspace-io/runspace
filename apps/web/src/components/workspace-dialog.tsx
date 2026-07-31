import { Check, Plus, X } from 'lucide-react';
import type { WorkspaceSummary } from '@/lib/workspace-state';
import { useModalFocus } from './use-modal-focus';

export type DialogMode = 'list' | 'create';
export type WorkspaceDialogProps = {
  mode: DialogMode;
  workspaces: readonly WorkspaceSummary[];
  activeWorkspace: WorkspaceSummary | undefined;
  workspaceName: string;
  error?: string | undefined;
  onClose: () => void;
  onModeChange: (mode: DialogMode) => void;
  onWorkspaceNameChange: (value: string) => void;
  onSelectWorkspace: (workspace: WorkspaceSummary) => void;
  onCreateWorkspace: (name?: string) => void;
};

export function WorkspaceDialog({
  mode,
  workspaces,
  activeWorkspace,
  workspaceName,
  error,
  onClose,
  onModeChange,
  onWorkspaceNameChange,
  onSelectWorkspace,
  onCreateWorkspace,
}: WorkspaceDialogProps) {
  const dialogRef = useModalFocus(true, onClose);
  const title = mode === 'list' ? 'Switch workspace' : 'Create workspace';
  return (
    <div
      className="dialog-backdrop"
      role="presentation"
      onMouseDown={(event) => event.target === event.currentTarget && onClose()}
    >
      <section
        ref={dialogRef}
        className="workspace-dialog"
        role="dialog"
        data-testid="workspace-dialog"
        aria-modal="true"
        aria-labelledby="workspace-dialog-title"
      >
        <div className="dialog-header">
          <div>
            <p className="eyebrow">WORKSPACE CONTROL PLANE</p>
            <h2 id="workspace-dialog-title">{title}</h2>
          </div>
          <button className="icon-button" aria-label="Close workspace dialog" onClick={onClose}>
            <X size={17} />
          </button>
        </div>
        {mode === 'list' && (
          <WorkspaceList
            workspaces={workspaces}
            activeWorkspace={activeWorkspace}
            onSelect={onSelectWorkspace}
            onModeChange={onModeChange}
          />
        )}
        {mode === 'create' && (
          <WorkspaceForm
            name={workspaceName}
            error={error}
            onNameChange={onWorkspaceNameChange}
            onBack={() => onModeChange('list')}
            onSubmit={(name) => onCreateWorkspace(name)}
          />
        )}
      </section>
    </div>
  );
}

function WorkspaceList({
  workspaces,
  activeWorkspace,
  onSelect,
  onModeChange,
}: {
  workspaces: readonly WorkspaceSummary[];
  activeWorkspace: WorkspaceSummary | undefined;
  onSelect: (workspace: WorkspaceSummary) => void;
  onModeChange: (mode: DialogMode) => void;
}) {
  return (
    <>
      <div className="dialog-list" aria-label="Available workspaces">
        {workspaces.length === 0 && (
          <p className="dialog-empty">No workspaces yet. Create one to get started.</p>
        )}
        {workspaces.map((workspace) => (
          <button
            className={`dialog-list-item ${workspace.id === activeWorkspace?.id ? 'selected' : ''}`}
            key={workspace.id}
            onClick={() => onSelect(workspace)}
          >
            <span className="workspace-avatar">{workspace.name.slice(0, 1).toUpperCase()}</span>
            <span>
              <strong>{workspace.name}</strong>
              <small>
                {workspace.resourceCount} resources · {workspace.slug}
              </small>
            </span>
            {workspace.id === activeWorkspace?.id && <Check size={16} />}
          </button>
        ))}
      </div>
      <div className="dialog-actions">
        <button
          type="button"
          className="dialog-primary"
          data-testid="new-workspace-button"
          onClick={(event) => {
            event.preventDefault();
            onModeChange('create');
          }}
        >
          <Plus size={15} /> New workspace
        </button>
      </div>
    </>
  );
}

function WorkspaceForm({
  name,
  error,
  onNameChange,
  onBack,
  onSubmit,
}: {
  name: string;
  error?: string | undefined;
  onNameChange: (value: string) => void;
  onBack: () => void;
  onSubmit: (name: string) => void;
}) {
  return (
    <form
      className="dialog-form"
      onSubmit={(event) => {
        event.preventDefault();
        const data = new FormData(event.currentTarget);
        onSubmit(String(data.get('workspace-name') ?? ''));
      }}
    >
      <label htmlFor="workspace-name">Workspace name</label>
      <input
        id="workspace-name"
        autoFocus
        value={name}
        name="workspace-name"
        onChange={(event) => onNameChange(event.target.value)}
        placeholder="e.g. Atlas"
        maxLength={64}
      />
      <span className="field-hint">
        A workspace is where your resources, runs, and collaborators live.
      </span>
      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}
      <div className="dialog-actions">
        <button type="button" className="dialog-secondary" onClick={onBack}>
          Back
        </button>
        <button type="submit" className="dialog-primary">
          Create workspace
        </button>
      </div>
    </form>
  );
}
