'use client';

import { useCallback, useEffect, useState } from 'react';
import type { WorkspaceApiClient } from '@/lib/api-client';
import { discoverLocalAgents, type LocalAgentInstallation } from '@/lib/host-agent-client';

export function useLocalAgentDiscovery(api: WorkspaceApiClient, initialID: string) {
  const [agents, setAgents] = useState<LocalAgentInstallation[]>([]);
  const [selectedID, setSelectedID] = useState(initialID);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();

  const discover = useCallback(async () => {
    setLoading(true);
    setError(undefined);
    try {
      const local = await discoverLocalAgents(api.actorID);
      await Promise.all(local.map((agent) => api.assignLocalAgent(agent)));
      setAgents(local);
      const selected = local.find((agent) => agent.id === selectedID);
      const fallback = selected ?? local.find((agent) => agent.status === 'ready');
      if (fallback) setSelectedID(fallback.id);
    } catch (discoveryError) {
      setError(
        discoveryError instanceof Error ? discoveryError.message : 'Agent discovery failed.',
      );
    } finally {
      setLoading(false);
    }
  }, [api, selectedID]);

  useEffect(() => {
    void discover();
    // Scan once when the connection dialog opens; refresh is explicit after that.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return { agents, selectedID, setSelectedID, loading, error, setError, discover };
}
