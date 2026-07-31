'use client';

import { useCallback, useEffect, useState, type FormEvent } from 'react';
import { KeyRound, Trash2, X } from 'lucide-react';
import { WorkspaceApiClient, type ApiChannel, type ApiSecretMetadata } from '@/lib/api-client';
import type { ChannelSettingsDraft } from './channel-model';
import { channelSettingsDraft } from './channel-config';
import { useModalFocus } from './use-modal-focus';

export function ChannelSettingsDialog({
  api,
  channel,
  onClose,
  onUpdate,
}: {
  api: WorkspaceApiClient;
  channel: ApiChannel;
  onClose: () => void;
  onUpdate: (draft: ChannelSettingsDraft) => Promise<boolean>;
}) {
  const dialogRef = useModalFocus(true, onClose);
  const [name, setName] = useState(channel.name);
  const [error, setError] = useState<string>();
  const [saving, setSaving] = useState(false);
  const saveSettings = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!name.trim()) return setError('Channel name is required.');
    setSaving(true);
    const updated = await onUpdate(channelSettingsDraft(channel, { name }));
    setSaving(false);
    if (updated) onClose();
  };
  return (
    <div
      className="dialog-backdrop"
      role="presentation"
      onMouseDown={(event) => event.target === event.currentTarget && onClose()}
    >
      <section
        ref={dialogRef}
        className="workspace-dialog channel-settings-dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby="channel-settings-title"
      >
        <div className="dialog-header">
          <div>
            <p className="eyebrow">CHANNEL CONTROL PLANE</p>
            <h2 id="channel-settings-title">{channel.name}</h2>
          </div>
          <button className="icon-button" aria-label="Close channel settings" onClick={onClose}>
            <X size={17} />
          </button>
        </div>
        <form className="channel-form" onSubmit={saveSettings}>
          <label className="channel-field">
            <span>Channel name</span>
            <input value={name} maxLength={64} onChange={(event) => setName(event.target.value)} />
          </label>
          <p className="settings-routing-note">
            Resource and agent connections are managed from the channel context sidebar.
          </p>
          <div className="dialog-actions">
            <button type="button" className="dialog-secondary" onClick={onClose}>
              Cancel
            </button>
            <button type="submit" className="dialog-primary" disabled={saving}>
              {saving ? 'Saving…' : 'Save settings'}
            </button>
          </div>
        </form>
        <ChannelSecrets api={api} channelID={channel.id} />
        {error && <p className="dialog-error">{error}</p>}
      </section>
    </div>
  );
}

function ChannelSecrets({ api, channelID }: { api: WorkspaceApiClient; channelID: string }) {
  const [secrets, setSecrets] = useState<ApiSecretMetadata[]>([]);
  const [secretName, setSecretName] = useState('');
  const [secretValue, setSecretValue] = useState('');
  const [error, setError] = useState<string>();
  const loadSecrets = useCallback(
    () =>
      api
        .listChannelSecrets(channelID)
        .then(setSecrets)
        .catch(() => setError('Unable to load channel secrets.')),
    [api, channelID],
  );
  useEffect(() => {
    void loadSecrets();
  }, [loadSecrets]);
  const save = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!secretName.trim() || !secretValue) return setError('Secret name and value are required.');
    try {
      await api.setChannelSecret(channelID, secretName.trim(), secretValue);
      setSecretName('');
      setSecretValue('');
      setError(undefined);
      await loadSecrets();
    } catch {
      setError('Unable to save this secret.');
    }
  };
  const remove = async (secret: ApiSecretMetadata) => {
    try {
      await api.deleteChannelSecret(channelID, secret.name);
      setSecrets((current) => current.filter((item) => item.name !== secret.name));
      setError(undefined);
    } catch {
      setError('This secret is inherited or could not be deleted here.');
    }
  };
  return (
    <section className="secret-section" aria-labelledby="channel-secrets-title">
      <div>
        <p className="eyebrow">SHARED SECRET STORE</p>
        <h3 id="channel-secrets-title">Available to channel agents</h3>
      </div>
      <div className="secret-list">
        {secrets.map((secret) => (
          <div className="secret-row" key={secret.name}>
            <KeyRound size={14} />
            <span>{secret.name}</span>
            <time>
              {secret.inherited ? 'Inherited' : new Date(secret.updated_at).toLocaleDateString()}
            </time>
            {secret.inherited ? (
              <span />
            ) : (
              <button
                className="tiny-button"
                aria-label={`Delete ${secret.name}`}
                onClick={() => void remove(secret)}
              >
                <Trash2 size={13} />
              </button>
            )}
          </div>
        ))}
        {secrets.length === 0 && <p className="empty-state">No secrets configured</p>}
      </div>
      <form className="secret-form" onSubmit={save}>
        <input
          aria-label="Secret name"
          value={secretName}
          onChange={(event) => setSecretName(event.target.value)}
          placeholder="OPENAI_API_KEY"
        />
        <input
          aria-label="Secret value"
          type="password"
          value={secretValue}
          onChange={(event) => setSecretValue(event.target.value)}
          placeholder="Secret value"
        />
        <button className="dialog-secondary" type="submit">
          Add secret
        </button>
      </form>
      {error && <p className="dialog-error">{error}</p>}
    </section>
  );
}
