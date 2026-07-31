'use client';

import { useEffect, useState, type FormEvent } from 'react';
import { ShieldCheck, UserPlus, X } from 'lucide-react';
import { WorkspaceApiClient, type ApiMember } from '@/lib/api-client';
import type { WorkspaceSummary } from '@/lib/workspace-state';
import { useModalFocus } from './use-modal-focus';

export function MembersDialog({
  api,
  workspace,
  onClose,
}: {
  api: WorkspaceApiClient;
  workspace: WorkspaceSummary;
  onClose: () => void;
}) {
  const dialogRef = useModalFocus(true, onClose);
  const [members, setMembers] = useState<ApiMember[]>([]);
  const [userID, setUserID] = useState('');
  const [role, setRole] = useState<'member' | 'viewer'>('member');
  const [error, setError] = useState<string>();
  const [saving, setSaving] = useState(false);
  useEffect(() => {
    let active = true;
    void api
      .listMembers(workspace.id)
      .then((items) => active && setMembers(items))
      .catch(() => active && setError('Unable to load workspace members.'));
    return () => {
      active = false;
    };
  }, [api, workspace.id]);
  const add = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!userID.trim()) return setError('Member ID is required.');
    setSaving(true);
    try {
      const member = await api.addMember(workspace.id, userID.trim(), role);
      setMembers((current) => [...current, member]);
      setUserID('');
      setError(undefined);
    } catch {
      setError('Unable to add this member. They may already belong to the workspace.');
    } finally {
      setSaving(false);
    }
  };
  return (
    <div
      className="dialog-backdrop"
      role="presentation"
      onMouseDown={(event) => event.target === event.currentTarget && onClose()}
    >
      <section
        ref={dialogRef}
        className="workspace-dialog members-dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby="members-dialog-title"
      >
        <div className="dialog-header">
          <div>
            <p className="eyebrow">WORKSPACE ACCESS</p>
            <h2 id="members-dialog-title">Members of {workspace.name}</h2>
          </div>
          <button className="icon-button" aria-label="Close members" onClick={onClose}>
            <X size={17} />
          </button>
        </div>
        <div className="member-list">
          {members.map((member) => (
            <div className="member-row" key={member.user_id}>
              <span className="member-avatar">{member.user_id.slice(0, 1).toUpperCase()}</span>
              <span>
                <strong>{member.user_id}</strong>
                <small>{member.role}</small>
              </span>
              {member.role === 'owner' && <ShieldCheck size={15} aria-label="Workspace owner" />}
            </div>
          ))}
          {members.length === 0 && <p className="empty-state">No members returned</p>}
        </div>
        <form className="member-form" onSubmit={add}>
          <label className="channel-field">
            <span>Member ID</span>
            <input
              value={userID}
              onChange={(event) => setUserID(event.target.value)}
              placeholder="alice"
            />
          </label>
          <label className="channel-field">
            <span>Role</span>
            <select
              value={role}
              onChange={(event) => setRole(event.target.value as 'member' | 'viewer')}
            >
              <option value="member">Member</option>
              <option value="viewer">Viewer</option>
            </select>
          </label>
          <button className="dialog-primary" type="submit" disabled={saving}>
            <UserPlus size={14} /> {saving ? 'Adding…' : 'Add member'}
          </button>
        </form>
        {error && <p className="dialog-error">{error}</p>}
      </section>
    </div>
  );
}
