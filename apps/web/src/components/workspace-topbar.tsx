import Image from 'next/image';
import { ChevronDown, LogOut, Menu, Users } from 'lucide-react';

export function WorkspaceTopbar({
  workspaceName,
  connected,
  onOpenNavigation,
  onOpenWorkspace,
  onOpenMembers,
  onSignOut,
}: {
  workspaceName?: string | undefined;
  connected: boolean;
  onOpenNavigation: () => void;
  onOpenWorkspace: () => void;
  onOpenMembers: () => void;
  onSignOut: () => void;
}) {
  return (
    <header className="topbar">
      <div className="brand-lockup">
        <button
          className="icon-button mobile-only"
          aria-label="Open navigation"
          onClick={onOpenNavigation}
        >
          <Menu size={18} />
        </button>
        <Image
          className="brand-mark"
          src="/brand/runspace-icon.svg"
          width={24}
          height={24}
          alt=""
        />
        <Image
          className="brand-wordmark"
          src="/brand/runspace-wordmark.svg"
          width={92}
          height={20}
          alt="Runspace"
          priority
        />
        <span className="brand-divider" />
        <button
          className="workspace-switcher"
          aria-label="Switch workspace"
          onClick={onOpenWorkspace}
        >
          <span className={`status-dot ${connected ? 'online' : ''}`} />
          <span>{workspaceName ?? 'Select workspace'}</span>
          <ChevronDown size={14} />
        </button>
      </div>
      <div className="topbar-actions">
        <button
          className="icon-button"
          aria-label="Members"
          title="Members"
          disabled={!workspaceName}
          onClick={onOpenMembers}
        >
          <Users size={16} />
        </button>
        <button className="icon-button" aria-label="Sign out" title="Sign out" onClick={onSignOut}>
          <LogOut size={16} />
        </button>
      </div>
    </header>
  );
}
