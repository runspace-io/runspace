'use client';

import { useEffect, useState } from 'react';
import { Container, Shield, Terminal } from 'lucide-react';
import { getHostAgentStatus, type HostAgentStatus } from '@/lib/host-agent-client';
import type { RepositorySummary } from '@/lib/workspace-state';
import { ConnectionDialog } from './connection-dialog';

export function TerminalLaunchDialog({
  repository,
  onClose,
  onLaunch,
}: {
  repository: RepositorySummary;
  onClose: () => void;
  onLaunch: (target: 'workspace' | 'host', level?: HostAgentStatus['access_level']) => void;
}) {
  const [host, setHost] = useState<HostAgentStatus>();
  const [hostError, setHostError] = useState<string>();
  const local = repository.provider === 'mirror' || repository.provider === 'folder';
  useEffect(() => {
    if (!local) return;
    void getHostAgentStatus()
      .then(setHost)
      .catch((error: unknown) =>
        setHostError(error instanceof Error ? error.message : 'Host Agent is unavailable.'),
      );
  }, [local]);
  return (
    <ConnectionDialog
      eyebrow="TERMINAL ACCESS"
      title="Open terminal"
      description={`Choose where the shell for ${repository.fullName} should run.`}
      icon={<Terminal size={18} />}
      onClose={onClose}
    >
      <div className="terminal-launch-options">
        {!local && (
          <button onClick={() => onLaunch('workspace')}>
            <Container size={17} />
            <span>
              <strong>Workspace terminal</strong>
              <small>Docker sandbox · Git checkout</small>
            </span>
          </button>
        )}
        {local && (
          <button
            disabled={!host || host.access_level !== 'user'}
            onClick={() => onLaunch('host', 'user')}
          >
            <Terminal size={17} />
            <span>
              <strong>Host terminal · User</strong>
              <small>Runs on your computer inside the approved folder</small>
            </span>
          </button>
        )}
        {local && (
          <button disabled={!host?.elevated} onClick={() => onLaunch('host', 'administrator')}>
            <Shield size={17} />
            <span>
              <strong>Host terminal · Administrator</strong>
              <small>
                {host?.elevated
                  ? 'Host Agent is elevated'
                  : 'Restart Host Agent as Administrator to enable'}
              </small>
            </span>
          </button>
        )}
        {hostError && <p className="terminal-launch-error">{hostError}</p>}
      </div>
    </ConnectionDialog>
  );
}
