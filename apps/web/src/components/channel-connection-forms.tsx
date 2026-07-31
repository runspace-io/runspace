'use client';

import { useState, type FormEvent } from 'react';
import { FolderGit2, GitBranch, Link2, Plus } from 'lucide-react';
import type { RepositorySummary } from '@/lib/workspace-state';
import {
  initializeLocalRepository,
  inspectLocalRepository,
  type LocalRepositoryStatus,
} from '@/lib/host-agent-client';
import { ConnectionMethodPicker, type ConnectionMethod } from './connection-dialog';
import { LocalRepositoryField } from './local-repository-field';
import { resolveConnectionTargets } from './repository-connection-targets';

export type RepositoryMethod = 'existing' | 'git' | 'local';
const repositoryMethods: readonly ConnectionMethod<RepositoryMethod>[] = [
  {
    id: 'existing',
    label: 'Workspace',
    description: 'Already connected',
    icon: <GitBranch size={15} />,
  },
  {
    id: 'git',
    label: 'Remote Git',
    description: 'Git resource over HTTPS',
    icon: <Link2 size={15} />,
  },
  {
    id: 'local',
    label: 'Local folder',
    description: 'Host filesystem',
    icon: <FolderGit2 size={15} />,
  },
];

export function RepositoryConnectionForm({
  available,
  onConnect,
  onDone,
}: {
  available: readonly RepositorySummary[];
  onConnect: (
    repositoryIDs: readonly string[],
    repositoryURLs: readonly string[],
  ) => Promise<boolean>;
  onDone: () => void;
}) {
  const [repositoryIDs, setRepositoryIDs] = useState<string[]>([]);
  const [repositoryURL, setRepositoryURL] = useState('');
  const [localPath, setLocalPath] = useState('');
  const [localStatus, setLocalStatus] = useState<LocalRepositoryStatus>();
  const [inspecting, setInspecting] = useState(false);
  const [localError, setLocalError] = useState<string>();
  const [localPaths, setLocalPaths] = useState<string[]>([]);
  const [connectedCount, setConnectedCount] = useState(0);
  const [method, setMethod] = useState<RepositoryMethod>(available.length > 0 ? 'existing' : 'git');
  const [saving, setSaving] = useState(false);
  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setSaving(true);
    const targets = await resolveConnectionTargets({
      method,
      repositoryIDs,
      repositoryURL,
      localPath,
      localPaths,
      inspectPath,
    });
    const connected = targets
      ? await onConnect(targets.repositoryIDs, targets.repositoryURLs)
      : false;
    setSaving(false);
    if (!connected || !targets) return;
    setConnectedCount(
      (count) => count + Math.max(targets.repositoryIDs.length, targets.repositoryURLs.length),
    );
    if (method === 'existing' && targets.repositoryIDs.length === available.length) {
      setMethod('git');
    }
    setRepositoryIDs([]);
    setRepositoryURL('');
    setLocalPath('');
    setLocalPaths([]);
    setLocalStatus(undefined);
  };
  const inspectPath = async (path: string) => {
    if (!path.trim()) return undefined;
    setInspecting(true);
    setLocalError(undefined);
    try {
      const status = await inspectLocalRepository(path.trim());
      setLocalStatus(status);
      return status;
    } catch (error) {
      setLocalStatus(undefined);
      setLocalError(error instanceof Error ? error.message : 'Could not inspect this folder.');
      return undefined;
    } finally {
      setInspecting(false);
    }
  };
  const initializeGit = async () => {
    setInspecting(true);
    setLocalError(undefined);
    try {
      setLocalStatus(await initializeLocalRepository(localPath.trim()));
    } catch (error) {
      setLocalError(error instanceof Error ? error.message : 'Could not initialize Git.');
    } finally {
      setInspecting(false);
    }
  };
  const queueLocalPath = async () => {
    const status = await inspectPath(localPath);
    if (!status?.can_connect) return;
    setLocalPaths((paths) => [...new Set([...paths, status.path])]);
    setLocalPath('');
    setLocalStatus(undefined);
  };
  return (
    <form className="connection-dialog-form" onSubmit={submit}>
      <ConnectionMethodPicker
        label="Resource source"
        methods={available.length > 0 ? repositoryMethods : repositoryMethods.slice(1)}
        value={method}
        onChange={setMethod}
      />
      <RepositoryMethodFields
        method={method}
        available={available}
        repositoryIDs={repositoryIDs}
        repositoryURL={repositoryURL}
        localPath={localPath}
        localStatus={localStatus}
        inspecting={inspecting}
        localError={localError}
        localPaths={localPaths}
        onRepositoryIDsChange={setRepositoryIDs}
        onRepositoryURLChange={setRepositoryURL}
        onLocalPathChange={(value) => {
          setLocalPath(value);
          setLocalStatus(undefined);
          setLocalError(undefined);
        }}
        onInspectLocalPath={() => void inspectPath(localPath)}
        onInitializeGit={() => void initializeGit()}
        onQueueLocalPath={() => void queueLocalPath()}
        onRemoveLocalPath={(path) =>
          setLocalPaths((paths) => paths.filter((queuedPath) => queuedPath !== path))
        }
      />
      <button
        className="context-primary-action"
        disabled={
          saving ||
          repositoryConnectionMissing(method, repositoryIDs, repositoryURL, localPath, localPaths)
        }
      >
        <Link2 size={14} />
        {saving ? 'Connecting…' : 'Connect resource'}
      </button>
      <div className="connection-dialog-footer">
        <span>
          {connectedCount > 0 ? `${connectedCount} added in this session` : 'Add one or many'}
        </span>
        <button type="button" className="context-secondary-action" onClick={onDone}>
          Done
        </button>
      </div>
    </form>
  );
}

function RepositoryMethodFields({
  method,
  available,
  repositoryIDs,
  repositoryURL,
  localPath,
  localStatus,
  inspecting,
  localError,
  localPaths,
  onRepositoryIDsChange,
  onRepositoryURLChange,
  onLocalPathChange,
  onInspectLocalPath,
  onInitializeGit,
  onQueueLocalPath,
  onRemoveLocalPath,
}: {
  method: RepositoryMethod;
  available: readonly RepositorySummary[];
  repositoryIDs: readonly string[];
  repositoryURL: string;
  localPath: string;
  localStatus: LocalRepositoryStatus | undefined;
  inspecting: boolean;
  localError: string | undefined;
  localPaths: readonly string[];
  onRepositoryIDsChange: (value: string[]) => void;
  onRepositoryURLChange: (value: string) => void;
  onLocalPathChange: (value: string) => void;
  onInspectLocalPath: () => void;
  onInitializeGit: () => void;
  onQueueLocalPath: () => void;
  onRemoveLocalPath: (path: string) => void;
}) {
  if (method === 'existing') {
    return (
      <fieldset className="repository-checklist">
        <legend>Workspace resources</legend>
        {available.map((repository) => (
          <label key={repository.id}>
            <input
              type="checkbox"
              checked={repositoryIDs.includes(repository.id)}
              onChange={() =>
                onRepositoryIDsChange(
                  repositoryIDs.includes(repository.id)
                    ? repositoryIDs.filter((id) => id !== repository.id)
                    : [...repositoryIDs, repository.id],
                )
              }
            />
            <span>
              <strong>{repository.fullName}</strong>
              <small>{repository.defaultBranch}</small>
            </span>
          </label>
        ))}
      </fieldset>
    );
  }
  if (method === 'git') {
    return (
      <label>
        <span>Git HTTPS URL</span>
        <input
          aria-label="Git HTTPS URL"
          value={repositoryURL}
          onChange={(event) => onRepositoryURLChange(event.target.value)}
          placeholder="https://github.com/org/repo"
        />
      </label>
    );
  }
  return (
    <>
      <LocalRepositoryField
        path={localPath}
        status={localStatus}
        inspecting={inspecting}
        onPathChange={onLocalPathChange}
        onInspect={onInspectLocalPath}
        onInitialize={onInitializeGit}
        queuedPaths={localPaths}
        onQueue={onQueueLocalPath}
        onRemoveQueued={onRemoveLocalPath}
      />
      {localError && <small className="connection-field-error">{localError}</small>}
    </>
  );
}

function repositoryConnectionMissing(
  method: RepositoryMethod,
  repositoryIDs: readonly string[],
  remoteURL: string,
  localPath: string,
  localPaths: readonly string[],
): boolean {
  if (method === 'existing') return repositoryIDs.length === 0;
  if (method === 'git') return !remoteURL.trim();
  return !localPath.trim() && localPaths.length === 0;
}

export function ContextAddButton({ label, onClick }: { label: string; onClick: () => void }) {
  return (
    <button className="context-add-button" onClick={onClick}>
      <Plus size={13} />
      {label}
    </button>
  );
}
