'use client';

import { useEffect, useState } from 'react';
import { X } from 'lucide-react';
import type { ApiMember, ApiTaskGrant, WorkspaceApiClient } from '@/lib/api-client';
import { errorMessage } from './agent-task-controller';

export function TaskAccessPanel({
  api,
  workspaceID,
  taskID,
  agentID,
  onClose,
}: {
  api: WorkspaceApiClient;
  workspaceID: string;
  taskID: string;
  agentID: string;
  onClose: () => void;
}) {
  const access = useTaskAccess(api, workspaceID, taskID, agentID);
  return (
    <aside className="agent-task-access-panel" aria-label="Chat access">
      <header>
        <div>
          <span className="eyebrow">CHAT-SCOPED AUTHORITY</span>
          <h3>Team access</h3>
        </div>
        <button className="icon-button" aria-label="Close chat access" onClick={onClose}>
          <X size={15} />
        </button>
      </header>
      <p>
        Share control of this chat, not access to your computer. Local paths, credentials, and raw
        ACP context stay private.
      </p>
      <TaskAccessForm access={access} />
      {access.error && <p className="agent-task-access-error">{access.error}</p>}
      <TaskGrantList grants={access.grants} />
      <footer>
        Cross-device instructions use the owner Host Agent transport. The owner device must be
        online.
      </footer>
    </aside>
  );
}

function TaskAccessForm({ access }: { access: ReturnType<typeof useTaskAccess> }) {
  if (access.members.length === 0) {
    return (
      <div className="agent-task-access-empty">Add workspace members before sharing a chat.</div>
    );
  }
  return (
    <div className="agent-task-access-form">
      <label>
        <span>Member</span>
        <select
          aria-label="Chat member"
          value={access.principalID}
          onChange={(event) => access.setPrincipalID(event.target.value)}
        >
          {access.members.map((member) => (
            <option key={member.user_id} value={member.user_id}>
              {member.user_id}
            </option>
          ))}
        </select>
      </label>
      <label>
        <span>Role</span>
        <select
          aria-label="Chat role"
          value={access.role}
          onChange={(event) => access.setRole(event.target.value as ApiTaskGrant['role'])}
        >
          <option value="viewer">Viewer</option>
          <option value="contributor">Contributor</option>
          <option value="operator">Operator</option>
          <option value="approver">Approver</option>
        </select>
      </label>
      <button
        className="dialog-primary"
        type="button"
        disabled={!access.principalID || access.saving}
        onClick={() => void access.grant()}
      >
        {access.saving ? 'Granting…' : 'Grant access'}
      </button>
    </div>
  );
}

function TaskGrantList({ grants }: { grants: readonly ApiTaskGrant[] }) {
  return (
    <div className="agent-task-grants">
      <span>Active grants</span>
      {grants.length === 0 ? (
        <p>No one else can interact with this chat.</p>
      ) : (
        <ul>
          {grants.map((grant) => (
            <li key={grant.principal_id}>
              <span>
                <strong>{grant.principal_id}</strong>
                <small>{grant.permissions.join(' · ')}</small>
              </span>
              <b>{grant.role}</b>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

function useTaskAccess(
  api: WorkspaceApiClient,
  workspaceID: string,
  taskID: string,
  agentID: string,
) {
  const [members, setMembers] = useState<ApiMember[]>([]);
  const [grants, setGrants] = useState<ApiTaskGrant[]>([]);
  const [principalID, setPrincipalID] = useState('');
  const [role, setRole] = useState<ApiTaskGrant['role']>('contributor');
  const [error, setError] = useState<string>();
  const [saving, setSaving] = useState(false);
  useEffect(() => {
    let active = true;
    void Promise.all([
      api.listMembers(workspaceID),
      api.listTaskGrants(workspaceID, taskID, agentID),
    ])
      .then(([nextMembers, nextGrants]) => {
        if (!active) return;
        const collaborators = nextMembers.filter((member) => member.user_id !== api.actorID);
        setMembers(collaborators);
        setGrants(nextGrants);
        setPrincipalID((current) => current || collaborators[0]?.user_id || '');
      })
      .catch((reason) => {
        if (active) setError(errorMessage(reason, 'Chat access could not load.'));
      });
    return () => {
      active = false;
    };
  }, [api, workspaceID, taskID, agentID]);
  const grant = async () => {
    if (!principalID || saving) return;
    setSaving(true);
    setError(undefined);
    try {
      const saved = await api.grantTaskAccess(workspaceID, taskID, agentID, principalID, role);
      setGrants((current) => [
        ...current.filter((item) => item.principal_id !== saved.principal_id),
        saved,
      ]);
    } catch (reason) {
      setError(errorMessage(reason, 'Task access could not be granted.'));
    } finally {
      setSaving(false);
    }
  };
  return {
    members,
    grants,
    principalID,
    setPrincipalID,
    role,
    setRole,
    error,
    saving,
    grant,
  };
}
