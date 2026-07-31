'use client';

import { useState, type FormEvent, type ReactNode } from 'react';
import { GitBranch, Network, X } from 'lucide-react';
import type { RepositorySummary } from '@/lib/workspace-state';
import { validateChannelDraft, type ChannelDraft } from './channel-model';
import { useModalFocus } from './use-modal-focus';

export function ChannelDialog({
  parentName,
  parentID = '',
  repositories,
  onClose,
  onCreate,
}: {
  parentName?: string | undefined;
  parentID?: string | undefined;
  repositories: readonly RepositorySummary[];
  onClose: () => void;
  onCreate: (draft: ChannelDraft) => Promise<boolean>;
}) {
  const dialogRef = useModalFocus(true, onClose);
  const [name, setName] = useState('');
  const [repositoryIDs, setRepositoryIDs] = useState<string[]>([]);
  const [repositoryURL, setRepositoryURL] = useState('');
  const [agentProtocol, setAgentProtocol] = useState<'none' | 'mock' | 'acp'>('none');
  const [agentCommand, setAgentCommand] = useState('');
  const [error, setError] = useState<string>();
  const [saving, setSaving] = useState(false);
  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const draft = {
      name,
      parentID,
      repositoryID: repositoryIDs[0] ?? '',
      repositoryIDs,
      repositoryURL,
      agentProtocol,
      agentCommand,
    };
    const validation = validateChannelDraft(draft);
    if (validation) return setError(validation);
    setSaving(true);
    const created = await onCreate(draft);
    setSaving(false);
    if (created) onClose();
  };
  return (
    <div
      className="dialog-backdrop"
      role="presentation"
      onMouseDown={(event) => event.target === event.currentTarget && onClose()}
    >
      <section
        ref={dialogRef}
        className="workspace-dialog channel-dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby="channel-dialog-title"
      >
        <div className="dialog-header">
          <div>
            <p className="eyebrow">{parentName ? 'NEW SUBCHANNEL' : 'NEW CHANNEL'}</p>
            <h2 id="channel-dialog-title">
              {parentName ? `Inside ${parentName}` : 'Create channel'}
            </h2>
          </div>
          <button className="icon-button" aria-label="Close channel dialog" onClick={onClose}>
            <X size={17} />
          </button>
        </div>
        <form className="channel-form" onSubmit={submit}>
          <Field label="Channel name" hint="The shared conversation and execution context.">
            <input
              autoFocus
              value={name}
              maxLength={64}
              onChange={(event) => setName(event.target.value)}
              placeholder="engineering"
            />
          </Field>
          <Field
            label="Resources"
            hint={
              parentName
                ? 'Leave inherited to use the parent resources.'
                : 'Optional until code access is needed.'
            }
          >
            <div className="field-icon-control">
              <GitBranch size={15} />
              <select
                value={repositoryIDs}
                onChange={(event) => {
                  const values = Array.from(event.target.selectedOptions, (option) => option.value);
                  setRepositoryIDs(values);
                }}
                multiple
              >
                {repositories.length === 0 && (
                  <option value="">
                    {parentName ? 'Inherited from parent' : 'No resources connected'}
                  </option>
                )}
                {repositories.map((repository) => (
                  <option key={repository.id} value={repository.id}>
                    {repository.fullName}
                  </option>
                ))}
              </select>
            </div>
          </Field>
          <Field label="Connect new resource" hint="GitHub HTTPS URL or a mounted file:// path.">
            <input
              value={repositoryURL}
              onChange={(event) => setRepositoryURL(event.target.value)}
              placeholder="https://github.com/org/repository"
            />
          </Field>
          <AgentRuntimeFields
            inherited={Boolean(parentName)}
            protocol={agentProtocol}
            command={agentCommand}
            onProtocolChange={setAgentProtocol}
            onCommandChange={setAgentCommand}
          />
          {error && <p className="dialog-error">{error}</p>}
          <div className="dialog-actions">
            <button type="button" className="dialog-secondary" onClick={onClose}>
              Cancel
            </button>
            <button type="submit" className="dialog-primary" disabled={saving}>
              {saving ? 'Creating…' : 'Create channel'}
            </button>
          </div>
        </form>
      </section>
    </div>
  );
}

function AgentRuntimeFields({
  inherited,
  protocol,
  command,
  onProtocolChange,
  onCommandChange,
}: {
  inherited: boolean;
  protocol: ChannelDraft['agentProtocol'];
  command: string;
  onProtocolChange: (protocol: ChannelDraft['agentProtocol']) => void;
  onCommandChange: (command: string) => void;
}) {
  return (
    <>
      <Field
        label="Agent runtime"
        hint={
          inherited
            ? 'Leave inherited to use the parent agent.'
            : 'Choose an explicit runtime for agent mode.'
        }
      >
        <div className="field-icon-control">
          <Network size={15} />
          <select
            value={protocol}
            onChange={(event) =>
              onProtocolChange(event.target.value as ChannelDraft['agentProtocol'])
            }
          >
            <option value="none">{inherited ? 'Inherit from parent' : 'No agent'}</option>
            <option value="mock">Built-in development agent</option>
            <option value="acp">ACP command</option>
          </select>
        </div>
      </Field>
      {protocol === 'acp' && (
        <Field label="ACP command" hint="Executable available inside the gateway container.">
          <input
            value={command}
            onChange={(event) => onCommandChange(event.target.value)}
            placeholder="codex-acp"
          />
        </Field>
      )}
    </>
  );
}

function Field({ label, hint, children }: { label: string; hint: string; children: ReactNode }) {
  return (
    <label className="channel-field">
      <span>{label}</span>
      {children}
      <small>{hint}</small>
    </label>
  );
}
