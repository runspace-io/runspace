'use client';

import { useMemo, useState, type FormEvent, type ReactNode } from 'react';
import { GitBranch, Network, X } from 'lucide-react';
import type { RepositorySummary } from '@/lib/workspace-state';
import { validateChannelDraft, type ChannelDraft } from './channel-model';
import { useModalFocus } from './use-modal-focus';
import { DisclosureSection } from './disclosure-section';

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
          <ResourceSection
            parentName={parentName}
            repositories={repositories}
            repositoryIDs={repositoryIDs}
            repositoryURL={repositoryURL}
            onRepositoryIDsChange={setRepositoryIDs}
            onRepositoryURLChange={setRepositoryURL}
          />
          <AgentSection
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

function ResourceSection({
  parentName,
  repositories,
  repositoryIDs,
  repositoryURL,
  onRepositoryIDsChange,
  onRepositoryURLChange,
}: {
  parentName: string | undefined;
  repositories: readonly RepositorySummary[];
  repositoryIDs: string[];
  repositoryURL: string;
  onRepositoryIDsChange: (ids: string[]) => void;
  onRepositoryURLChange: (value: string) => void;
}) {
  const summary = useMemo(
    () => resourceSummary(parentName, repositories, repositoryIDs, repositoryURL),
    [parentName, repositories, repositoryIDs, repositoryURL],
  );
  return (
    <DisclosureSection
      label="Resources"
      summary={summary}
      defaultOpen={repositoryIDs.length > 0 || repositoryURL.trim() !== ''}
    >
      {repositories.length > 0 && (
        <Field
          label="Existing resources"
          hint={parentName ? 'Leave empty to use the parent resources.' : 'Optional.'}
        >
          <div className="field-icon-control">
            <GitBranch size={15} />
            <select
              value={repositoryIDs}
              onChange={(event) => {
                const values = Array.from(event.target.selectedOptions, (option) => option.value);
                onRepositoryIDsChange(values);
              }}
              multiple
            >
              {repositories.map((repository) => (
                <option key={repository.id} value={repository.id}>
                  {repository.fullName}
                </option>
              ))}
            </select>
          </div>
        </Field>
      )}
      <Field
        label="Connect a new Git resource"
        hint="A GitHub HTTPS URL, cloned into the shared workspace checkout. A local agent
          on your own device needs its own folder connected separately, after the channel exists."
      >
        <input
          value={repositoryURL}
          onChange={(event) => onRepositoryURLChange(event.target.value)}
          placeholder="https://github.com/org/repository"
        />
      </Field>
    </DisclosureSection>
  );
}

function resourceSummary(
  parentName: string | undefined,
  repositories: readonly RepositorySummary[],
  repositoryIDs: string[],
  repositoryURL: string,
): string {
  if (repositoryURL.trim()) return 'Connecting a new resource';
  if (repositoryIDs.length === 1) return '1 resource selected';
  if (repositoryIDs.length > 1) return `${repositoryIDs.length} resources selected`;
  if (parentName) return 'Inherits from parent';
  if (repositories.length === 0) return 'None yet — optional';
  return 'Not connected';
}

function AgentSection({
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
    <DisclosureSection
      label="Agent runtime"
      summary={agentSummary(inherited, protocol)}
      defaultOpen={protocol !== 'none'}
    >
      <Field
        label="Runtime"
        hint={
          inherited
            ? 'Leave inherited to use the parent agent.'
            : 'You can also connect an agent from the channel later.'
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
            <option value="acp">ACP command (advanced)</option>
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
    </DisclosureSection>
  );
}

function agentSummary(inherited: boolean, protocol: ChannelDraft['agentProtocol']): string {
  if (protocol === 'mock') return 'Built-in development agent';
  if (protocol === 'acp') return 'ACP command';
  return inherited ? 'Inherits from parent' : 'No agent — optional';
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
