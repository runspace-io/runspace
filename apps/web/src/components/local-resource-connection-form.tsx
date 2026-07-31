'use client';

import { Cloud, Database, Github } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import {
  connectCapabilityResource,
  discoverResourceAdapters,
  type ResourceAdapter,
} from '@/lib/resource-adapter-client';
import { ConnectionMethodPicker } from './connection-dialog';

type AdapterID = ResourceAdapter['manifest']['id'];

export function LocalResourceConnectionForm({
  userID,
  workspaceID,
  onConnected,
}: {
  userID: string;
  workspaceID: string;
  onConnected: () => void;
}) {
  const [adapters, setAdapters] = useState<ResourceAdapter[]>([]);
  const [adapterID, setAdapterID] = useState<AdapterID>('github-cli');
  const [title, setTitle] = useState('My GitHub');
  const [profile, setProfile] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    void discoverResourceAdapters(userID)
      .then(setAdapters)
      .catch((reason: unknown) => setError(errorMessage(reason)));
  }, [userID]);
  const selected = useMemo(
    () => adapters.find((adapter) => adapter.manifest.id === adapterID),
    [adapterID, adapters],
  );
  const methods = adapters.map((adapter) => ({
    id: adapter.manifest.id,
    label: adapter.manifest.name,
    description:
      adapter.status === 'ready'
        ? `${adapter.manifest.capabilities.length} local capabilities`
        : `${adapter.manifest.executable} not installed`,
    icon: adapterIcon(adapter.manifest.id),
  }));
  const connect = async () => {
    setBusy(true);
    setError('');
    try {
      await connectCapabilityResource({ userID, workspaceID, adapterID, title, profile });
      onConnected();
    } catch (reason) {
      setError(errorMessage(reason));
    } finally {
      setBusy(false);
    }
  };
  return (
    <>
      {methods.length ? (
        <ConnectionMethodPicker
          label="Local adapter"
          methods={methods}
          value={adapterID}
          onChange={(value) => {
            setAdapterID(value);
            setTitle(defaultTitle(value));
            setProfile('');
          }}
        />
      ) : (
        <p className="graph-loading">Discovering local tools…</p>
      )}
      <div className="capability-fields">
        <label>
          <span>Resource name</span>
          <input value={title} maxLength={120} onChange={(event) => setTitle(event.target.value)} />
        </label>
        {adapterID === 'postgresql' ? (
          <label>
            <span>PostgreSQL service profile</span>
            <input
              value={profile}
              maxLength={120}
              placeholder="production-readonly"
              onChange={(event) => setProfile(event.target.value)}
            />
            <small>Uses a local pg_service.conf profile; no password is uploaded.</small>
          </label>
        ) : null}
      </div>
      {selected ? (
        <div className="capability-contract">
          <strong>Owner-hosted · query only</strong>
          {selected.manifest.capabilities.map((capability) => (
            <span key={capability.id}>{capability.label}</span>
          ))}
        </div>
      ) : null}
      {error ? <p className="agent-task-error">{error}</p> : null}
      <button
        className="primary-button"
        disabled={busy || selected?.status !== 'ready' || !title.trim()}
        onClick={() => void connect()}
      >
        {busy ? 'Connecting…' : 'Connect local Resource'}
      </button>
    </>
  );
}

function defaultTitle(id: AdapterID) {
  if (id === 'postgresql') return 'PostgreSQL';
  if (id === 'digitalocean-cli') return 'DigitalOcean';
  return 'My GitHub';
}

function adapterIcon(id: AdapterID) {
  if (id === 'postgresql') return <Database size={18} />;
  if (id === 'digitalocean-cli') return <Cloud size={18} />;
  return <Github size={18} />;
}

function errorMessage(reason: unknown) {
  return reason instanceof Error ? reason.message : 'Could not connect this Resource.';
}
