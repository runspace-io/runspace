'use client';

import { useEffect, useRef, useState, type FormEvent } from 'react';
import { Bot, CheckCircle2, RefreshCw, ShieldAlert, Unplug } from 'lucide-react';
import type { WorkspaceApiClient } from '@/lib/api-client';
import {
  saveLocalAgentPreference,
  listLocalAgentModels,
  type LocalAgentInstallation,
} from '@/lib/host-agent-client';
import type { ChannelSettingsDraft } from './channel-model';
import { useLocalAgentDiscovery } from './use-local-agent-discovery';

type PermissionMode = 'default' | 'approve' | 'yolo';

export function AgentConnectionForm({
  api,
  initialCommand,
  onConnect,
  onConnected,
}: {
  api: WorkspaceApiClient;
  initialCommand: string;
  onConnect: (
    protocol: ChannelSettingsDraft['agentProtocol'],
    installationID: string,
  ) => Promise<boolean>;
  onConnected: () => void;
}) {
  const discovery = useLocalAgentDiscovery(api, initialCommand);
  const [model, setModel] = useState('');
  const [permissionMode, setPermissionMode] = useState<PermissionMode>('default');
  const [confirmedYolo, setConfirmedYolo] = useState(false);
  const [saving, setSaving] = useState(false);
  const [models, setModels] = useState<string[]>([]);
  const hydratedAgentID = useRef('');
  useEffect(() => {
    const selected = discovery.agents.find((agent) => agent.id === discovery.selectedID);
    if (!selected) return;
    if (hydratedAgentID.current !== selected.id) {
      hydratedAgentID.current = selected.id;
      setModel(selected.model ?? '');
      setPermissionMode(selected.permission_mode ?? 'default');
    }
    void listLocalAgentModels(api.actorID, selected.id)
      .then(setModels)
      .catch(() => setModels([]));
  }, [api.actorID, discovery.agents, discovery.selectedID]);

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const selected = discovery.agents.find((agent) => agent.id === discovery.selectedID);
    if (!selected || selected.status !== 'ready') return;
    if (permissionMode === 'yolo' && !confirmedYolo) return;
    setSaving(true);
    discovery.setError(undefined);
    try {
      await saveLocalAgentPreference(api.actorID, selected.id, {
        model: model.trim(),
        permission_mode: permissionMode,
      });
      await api.assignLocalAgent(selected);
      const connected = await onConnect('acp', selected.id);
      if (connected) onConnected();
    } catch (saveError) {
      discovery.setError(
        saveError instanceof Error ? saveError.message : 'Could not assign local agent.',
      );
    } finally {
      setSaving(false);
    }
  };

  const disconnect = async () => {
    setSaving(true);
    const disconnected = await onConnect('none', '');
    setSaving(false);
    if (disconnected) onConnected();
  };

  return (
    <form className="connection-dialog-form" onSubmit={submit}>
      <div className="agent-discovery-heading">
        <div>
          <strong>Agents on this PC</strong>
          <small>Commands, credentials, and sessions remain local.</small>
        </div>
        <button
          type="button"
          className="icon-button"
          aria-label="Scan again"
          onClick={discovery.discover}
        >
          <RefreshCw size={14} className={discovery.loading ? 'is-spinning' : ''} />
        </button>
      </div>
      <div className="agent-installation-list" role="radiogroup" aria-label="Discovered agents">
        {discovery.agents.map((agent) => (
          <AgentInstallationOption
            key={agent.id}
            agent={agent}
            selected={discovery.selectedID === agent.id}
            onSelect={() => {
              discovery.setSelectedID(agent.id);
              setModel(agent.model ?? '');
              setPermissionMode(agent.permission_mode ?? 'default');
              setConfirmedYolo(false);
            }}
          />
        ))}
        {!discovery.loading && discovery.agents.length === 0 && (
          <p className="connection-field-hint">No supported local agents were found.</p>
        )}
      </div>
      <AgentPreferenceFields
        model={model}
        models={models}
        permissionMode={permissionMode}
        confirmedYolo={confirmedYolo}
        onModelChange={setModel}
        onPermissionChange={(value) => {
          setPermissionMode(value);
          setConfirmedYolo(false);
        }}
        onConfirmYolo={setConfirmedYolo}
      />
      {discovery.error && <p className="connection-field-error">{discovery.error}</p>}
      <button
        className="context-primary-action"
        disabled={
          saving ||
          discovery.loading ||
          !discovery.agents.some(
            (agent) => agent.id === discovery.selectedID && agent.status === 'ready',
          ) ||
          (permissionMode === 'yolo' && !confirmedYolo)
        }
      >
        <Bot size={14} />
        {saving ? 'Saving…' : 'Set active collaborator'}
      </button>
      <button type="button" className="context-add-button" disabled={saving} onClick={disconnect}>
        <Unplug size={13} /> Disconnect agent
      </button>
    </form>
  );
}

function AgentPreferenceFields({
  model,
  models,
  permissionMode,
  confirmedYolo,
  onModelChange,
  onPermissionChange,
  onConfirmYolo,
}: {
  model: string;
  models: readonly string[];
  permissionMode: PermissionMode;
  confirmedYolo: boolean;
  onModelChange: (value: string) => void;
  onPermissionChange: (value: PermissionMode) => void;
  onConfirmYolo: (value: boolean) => void;
}) {
  return (
    <>
      <label>
        <span>Model</span>
        <input
          aria-label="Agent model"
          value={model}
          list="agent-model-suggestions"
          onChange={(event) => onModelChange(event.target.value)}
          placeholder="Agent default (or enter provider/model)"
          autoComplete="off"
        />
        <datalist id="agent-model-suggestions">
          {models.map((item) => (
            <option value={item} key={item} />
          ))}
        </datalist>
        <small className="connection-field-hint">
          Leave blank to use the agent&apos;s configured default.
        </small>
      </label>
      <label>
        <span>Permission mode</span>
        <select
          aria-label="Agent permission mode"
          value={permissionMode}
          onChange={(event) => onPermissionChange(event.target.value as PermissionMode)}
        >
          <option value="default">Agent default</option>
          <option value="approve">Safe — block unapproved actions</option>
          <option value="yolo">YOLO — skip permission prompts</option>
        </select>
      </label>
      {permissionMode === 'yolo' && (
        <label className="danger-confirmation">
          <input
            type="checkbox"
            checked={confirmedYolo}
            onChange={(event) => onConfirmYolo(event.target.checked)}
          />
          <ShieldAlert size={16} />
          <span>I understand this agent can execute commands and modify files without asking.</span>
        </label>
      )}
    </>
  );
}

function AgentInstallationOption({
  agent,
  selected,
  onSelect,
}: {
  agent: LocalAgentInstallation;
  selected: boolean;
  onSelect: () => void;
}) {
  return (
    <button
      type="button"
      role="radio"
      aria-checked={selected}
      className={`agent-installation ${selected ? 'is-selected' : ''}`}
      disabled={agent.status !== 'ready'}
      onClick={onSelect}
    >
      <Bot size={16} />
      <span>
        <strong>{agent.name}</strong>
        <small>
          {agent.status === 'ready' ? 'Ready via ACP' : 'Installed · ACP adapter required'}
        </small>
      </span>
      {agent.status === 'ready' && <CheckCircle2 size={14} />}
    </button>
  );
}
