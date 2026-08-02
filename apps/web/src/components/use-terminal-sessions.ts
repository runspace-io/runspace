'use client';

import { useEffect, useRef, useState } from 'react';
import type { WorkspaceApiClient } from '@/lib/api-client';
import type { RepositorySummary } from '@/lib/workspace-state';
import { hostTerminalURL, type HostAgentStatus } from '@/lib/host-agent-client';

export type TerminalSession = {
  id: string;
  repositoryID: string;
  repositoryName: string;
  url: string;
  target: 'workspace' | 'host';
  /** Only gateway terminals need a token; host terminals are loopback. */
  tokenSource?: (() => Promise<string>) | undefined;
  accessLevel?: HostAgentStatus['access_level'];
};

export function useTerminalSessions(api: WorkspaceApiClient, workspaceID: string | undefined) {
  const [sessions, setSessions] = useState<TerminalSession[]>([]);
  const [activeID, setActiveID] = useState<string>();
  const sequence = useRef(0);
  useEffect(() => {
    setSessions([]);
    setActiveID(undefined);
  }, [workspaceID]);
  const open = (
    repository: RepositorySummary | undefined,
    target: TerminalSession['target'] = 'workspace',
    accessLevel: HostAgentStatus['access_level'] = 'user',
  ) => {
    if (!workspaceID || !repository) return;
    sequence.current += 1;
    const id = `${repository.id}:${sequence.current}`;
    const session = {
      id,
      repositoryID: repository.id,
      repositoryName: repository.fullName,
      url:
        target === 'host'
          ? hostTerminalURL(repository.id, accessLevel, api.actorID)
          : api.terminalURL(workspaceID, repository.id),
      target,
      ...(target === 'host' ? { accessLevel } : { tokenSource: () => api.gatewayToken() }),
    };
    setSessions((current) => [...current, session]);
    setActiveID(id);
  };
  const close = (id: string) => {
    setSessions((current) => {
      const index = current.findIndex((session) => session.id === id);
      const next = current.filter((session) => session.id !== id);
      if (id === activeID) setActiveID(next[Math.max(0, index - 1)]?.id);
      return next;
    });
  };
  return { sessions, activeID, setActiveID, open, close };
}
