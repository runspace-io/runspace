'use client';

import { useEffect, useState } from 'react';
import { Check, Folder, GitBranch, LoaderCircle, Plus, X } from 'lucide-react';
import { suggestLocalPaths, type LocalRepositoryStatus } from '@/lib/host-agent-client';

export function LocalRepositoryField({
  path,
  status,
  inspecting,
  onPathChange,
  onInspect,
  onInitialize,
  queuedPaths,
  onQueue,
  onRemoveQueued,
}: {
  path: string;
  status: LocalRepositoryStatus | undefined;
  inspecting: boolean;
  onPathChange: (value: string) => void;
  onInspect: () => void;
  onInitialize: () => void;
  queuedPaths: readonly string[];
  onQueue: () => void;
  onRemoveQueued: (path: string) => void;
}) {
  const [suggestions, setSuggestions] = useState<string[]>([]);
  useEffect(() => {
    let active = true;
    const timeout = window.setTimeout(() => {
      void suggestLocalPaths(path)
        .then((paths) => {
          if (active) setSuggestions(paths);
        })
        .catch(() => {
          if (active) setSuggestions([]);
        });
    }, 180);
    return () => {
      active = false;
      window.clearTimeout(timeout);
    };
  }, [path]);
  return (
    <div className="local-repository-field">
      <label>
        <span>Local resource path</span>
        <input
          aria-label="Local resource path"
          value={path}
          onChange={(event) => onPathChange(event.target.value)}
          onBlur={(event) => {
            const next = event.relatedTarget;
            if (next instanceof HTMLButtonElement && next.form === event.currentTarget.form) return;
            onInspect();
          }}
          placeholder="D:\projects\my-repo or /home/me/projects/my-repo"
        />
      </label>
      {suggestions.length > 0 && (
        <div className="local-path-suggestions" role="listbox" aria-label="Folder suggestions">
          {suggestions.map((suggestion) => (
            <button
              type="button"
              role="option"
              aria-selected="false"
              key={suggestion}
              onClick={() => onPathChange(suggestion)}
            >
              {suggestion}
            </button>
          ))}
        </div>
      )}
      <RepositoryPreflight status={status} inspecting={inspecting} onInitialize={onInitialize} />
      {status?.can_connect && (
        <button type="button" className="local-path-queue-button" onClick={onQueue}>
          <Plus size={12} />
          Add path to batch
        </button>
      )}
      {queuedPaths.length > 0 && (
        <div className="local-path-queue" aria-label="Local resources to connect">
          {queuedPaths.map((queuedPath) => (
            <div key={queuedPath}>
              <span>{queuedPath}</span>
              <button
                type="button"
                aria-label={`Remove ${queuedPath}`}
                onClick={() => onRemoveQueued(queuedPath)}
              >
                <X size={11} />
              </button>
            </div>
          ))}
        </div>
      )}
      <small className="connection-field-hint">
        The Host Agent mirrors this folder into Runspace. No upload or Docker mount is required.
      </small>
    </div>
  );
}

function RepositoryPreflight({
  status,
  inspecting,
  onInitialize,
}: {
  status: LocalRepositoryStatus | undefined;
  inspecting: boolean;
  onInitialize: () => void;
}) {
  if (inspecting) {
    return (
      <div className="repository-preflight is-pending">
        <LoaderCircle size={13} className="spin" />
        Checking resource…
      </div>
    );
  }
  if (!status) return null;
  if (!status.git) {
    return (
      <div className="repository-preflight is-folder">
        <Folder size={13} />
        <span>Plain folder · ready</span>
        <button type="button" onClick={onInitialize}>
          Initialize Git (optional)
        </button>
      </div>
    );
  }
  return (
    <div className="repository-preflight is-ready">
      <Check size={13} />
      <span>{status.has_remote ? 'Git repository' : 'Git repository · no remote'}</span>
      <span className="repository-preflight-branch">
        <GitBranch size={11} />
        {status.branch}
      </span>
    </div>
  );
}
