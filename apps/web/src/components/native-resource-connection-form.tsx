'use client';

import { Cloud, Database, Github } from 'lucide-react';
import { useEffect, useMemo, useState, type ReactNode } from 'react';
import type { ApiResourcePlugin, WorkspaceApiClient } from '@/lib/api-client';
import { ConnectionMethodPicker } from './connection-dialog';

export function NativeResourceConnectionForm({
  api,
  workspaceID,
  onConnected,
}: {
  api: WorkspaceApiClient;
  workspaceID: string;
  onConnected: () => void;
}) {
  const [plugins, setPlugins] = useState<ApiResourcePlugin[]>([]);
  const [pluginID, setPluginID] = useState<ApiResourcePlugin['id']>('github');
  const [title, setTitle] = useState('My GitHub');
  const [credential, setCredential] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    void api
      .listResourcePlugins()
      .then(setPlugins)
      .catch((reason: unknown) => setError(errorMessage(reason)));
  }, [api]);
  const selected = useMemo(
    () => plugins.find((plugin) => plugin.id === pluginID),
    [pluginID, plugins],
  );
  const methods = plugins.map((plugin) => ({
    id: plugin.id,
    label: plugin.name,
    description: `${plugin.capabilities.length} native capabilities`,
    icon: pluginIcon(plugin.id),
  }));
  const connect = async () => {
    if (!selected) return;
    setBusy(true);
    setError('');
    try {
      await api.connectNativeResource(workspaceID, {
        plugin_id: pluginID,
        title,
        placement: 'runspace',
        auth_method: selected.auth_methods[0]?.id ?? 'token',
        access_mode: 'read',
        credential,
      });
      onConnected();
    } catch (reason) {
      setError(errorMessage(reason));
    } finally {
      setBusy(false);
    }
  };
  return (
    <>
      <PluginPicker
        methods={methods}
        value={pluginID}
        onChange={(value) => {
          setPluginID(value);
          setTitle(defaultTitle(value));
          setCredential('');
        }}
      />
      <div className="capability-fields">
        <label>
          <span>Resource name</span>
          <input value={title} maxLength={120} onChange={(event) => setTitle(event.target.value)} />
        </label>
        <label>
          <span>{authDetail(selected, 'secret_label', 'Credential')}</span>
          <input
            type="password"
            value={credential}
            autoComplete="off"
            placeholder={authDetail(selected, 'placeholder', '')}
            onChange={(event) => setCredential(event.target.value)}
          />
          <small>Encrypted before storage. It is never returned by Runspace APIs.</small>
        </label>
      </div>
      <NativeContract plugin={selected} />
      <ErrorNotice error={error} />
      <button
        className="primary-button"
        disabled={!canConnect(busy, selected, title, credential)}
        onClick={() => void connect()}
      >
        {connectLabel(busy)}
      </button>
    </>
  );
}

type PluginMethod = {
  id: ApiResourcePlugin['id'];
  label: string;
  description: string;
  icon: ReactNode;
};

function PluginPicker({
  methods,
  value,
  onChange,
}: {
  methods: PluginMethod[];
  value: ApiResourcePlugin['id'];
  onChange: (value: ApiResourcePlugin['id']) => void;
}) {
  if (!methods.length) return <p className="graph-loading">Loading native plugins…</p>;
  return (
    <ConnectionMethodPicker
      label="Native plugin"
      methods={methods}
      value={value}
      onChange={onChange}
    />
  );
}

function NativeContract({ plugin }: { plugin: ApiResourcePlugin | undefined }) {
  if (!plugin) return null;
  return (
    <div className="capability-contract">
      <strong>Runspace-hosted · query only</strong>
      {plugin.capabilities.map((capability) => (
        <span key={capability.id}>{capability.label}</span>
      ))}
    </div>
  );
}

function ErrorNotice({ error }: { error: string }) {
  return error ? <p className="agent-task-error">{error}</p> : null;
}

function authDetail(
  plugin: ApiResourcePlugin | undefined,
  key: 'secret_label' | 'placeholder',
  fallback: string,
) {
  return plugin?.auth_methods[0]?.[key] ?? fallback;
}

function canConnect(
  busy: boolean,
  plugin: ApiResourcePlugin | undefined,
  title: string,
  credential: string,
) {
  return !busy && Boolean(plugin) && Boolean(title.trim()) && Boolean(credential.trim());
}

function connectLabel(busy: boolean) {
  return busy ? 'Encrypting and connecting…' : 'Connect native Resource';
}

function defaultTitle(id: ApiResourcePlugin['id']) {
  if (id === 'postgresql') return 'PostgreSQL';
  if (id === 'digitalocean') return 'DigitalOcean';
  return 'My GitHub';
}

function pluginIcon(id: ApiResourcePlugin['id']) {
  if (id === 'postgresql') return <Database size={18} />;
  if (id === 'digitalocean') return <Cloud size={18} />;
  return <Github size={18} />;
}

function errorMessage(reason: unknown) {
  return reason instanceof Error ? reason.message : 'Could not connect this Resource.';
}
