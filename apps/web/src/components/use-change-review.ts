import { useCallback, useEffect, useState } from 'react';
import type { ApiRepositoryChange, ApiRepositoryDiff, WorkspaceApiClient } from '@/lib/api-client';

export function useChangeReview(
  api: WorkspaceApiClient,
  workspaceID: string,
  repositoryID: string,
) {
  const [changes, setChanges] = useState<ApiRepositoryChange[]>([]);
  const [selectedPath, setSelectedPath] = useState<string>();
  const [diff, setDiff] = useState<ApiRepositoryDiff>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();

  const loadChanges = useCallback(async () => {
    setLoading(true);
    setError(undefined);
    try {
      const items = await api.listChanges(workspaceID, repositoryID);
      setChanges(items);
      setSelectedPath((current) =>
        current && items.some((item) => item.path === current) ? current : items[0]?.path,
      );
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Unable to load resource changes.');
    } finally {
      setLoading(false);
    }
  }, [api, repositoryID, workspaceID]);

  useEffect(() => {
    void loadChanges();
  }, [loadChanges]);

  useEffect(() => {
    setDiff(undefined);
    if (!selectedPath) return;
    let active = true;
    void api
      .readDiff(workspaceID, repositoryID, selectedPath)
      .then((result) => active && setDiff(result))
      .catch((cause: unknown) => {
        if (active)
          setError(cause instanceof Error ? cause.message : 'Unable to load the selected diff.');
      });
    return () => {
      active = false;
    };
  }, [api, repositoryID, selectedPath, workspaceID]);

  return { changes, selectedPath, setSelectedPath, diff, loading, error, refresh: loadChanges };
}
