'use client';

import { useState } from 'react';
import { Check, Copy, Link2 } from 'lucide-react';
import type { WorkspaceApiClient } from '@/lib/api-client';
import type { ApiInvitation } from '@/lib/api-types';

/**
 * Creates a single-use invitation link.
 *
 * Adding a member by ID only works when you already know their exact identity,
 * which nobody does for a teammate who signs in through GitHub. The link is the
 * introduction; whoever opens it while signed in becomes the member.
 *
 * The token is returned once and never stored in readable form, so it is kept
 * in component state and shown until the dialog closes.
 */
export function InviteLink({ api, workspaceID }: { api: WorkspaceApiClient; workspaceID: string }) {
  const [role, setRole] = useState<ApiInvitation['role']>('member');
  const [link, setLink] = useState<string>();
  const [copied, setCopied] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();

  const create = async () => {
    setBusy(true);
    setError(undefined);
    setCopied(false);
    try {
      const { token } = await api.createInvitation(workspaceID, role);
      setLink(`${window.location.origin}/invite/${encodeURIComponent(token)}`);
    } catch {
      setError('Only workspace owners and admins can create invitation links.');
    } finally {
      setBusy(false);
    }
  };

  const copy = async () => {
    if (!link) return;
    try {
      await navigator.clipboard.writeText(link);
      setCopied(true);
    } catch {
      // Clipboard access can be denied; the link stays selectable in the input.
      setError('Could not copy automatically — select the link and copy it.');
    }
  };

  return (
    <div className="invite-link">
      <div className="invite-link-controls">
        <label className="channel-field">
          <span>Invite role</span>
          <select
            value={role}
            onChange={(event) => setRole(event.target.value as ApiInvitation['role'])}
          >
            <option value="member">Member</option>
            <option value="viewer">Viewer</option>
            <option value="admin">Admin</option>
          </select>
        </label>
        <button
          className="dialog-secondary"
          type="button"
          disabled={busy}
          onClick={() => void create()}
        >
          <Link2 size={14} /> {busy ? 'Creating…' : 'Create invite link'}
        </button>
      </div>
      {link && (
        <div className="invite-link-result">
          <input
            readOnly
            value={link}
            aria-label="Invitation link"
            onFocus={(e) => e.target.select()}
          />
          <button className="dialog-secondary" type="button" onClick={() => void copy()}>
            {copied ? <Check size={14} /> : <Copy size={14} />} {copied ? 'Copied' : 'Copy'}
          </button>
          <small>Single use, expires in 7 days. It is shown once — copy it now.</small>
        </div>
      )}
      {error && <p className="dialog-error">{error}</p>}
    </div>
  );
}
