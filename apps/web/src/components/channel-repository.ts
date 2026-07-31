import { useEffect, type Dispatch, type SetStateAction } from 'react';
import { WorkspaceApiClient, type ApiRepositoryFile } from '@/lib/api-client';
import { listHostRepositoryTree, readHostRepositoryFile } from '@/lib/host-agent-client';
import type { WorkspaceTreeEntry } from '@/lib/workspace-state';
import type { RepositorySummary } from '@/lib/workspace-state';

export function resourceHasGit(resource: RepositorySummary | undefined): boolean {
  return Boolean(resource) && resource?.provider !== 'folder';
}

export function loadRepositoryFile(input: {
  api: WorkspaceApiClient;
  workspaceID: string | undefined;
  repositoryID: string | undefined;
  repositoryProvider: string | undefined;
  path: string;
  setFile: (file: ApiRepositoryFile) => void;
  setError: (error: string) => void;
}) {
  const { api, workspaceID, repositoryID, repositoryProvider, path, setFile, setError } = input;
  if (!workspaceID || !repositoryID) return;
  void readRepositoryFile(api, workspaceID, repositoryID, repositoryProvider, path)
    .then(setFile)
    .catch(() => setError('Unable to read this resource file.'));
}

export function repositoryFileSelector(
  api: WorkspaceApiClient,
  workspaceID: string | undefined,
  repositoryID: string | undefined,
  repositoryProvider: string | undefined,
  state: {
    setFile: (file: ApiRepositoryFile) => void;
    setError: (error: string) => void;
  },
) {
  return (path: string) =>
    loadRepositoryFile({
      api,
      workspaceID,
      repositoryID,
      repositoryProvider,
      path,
      ...state,
    });
}

export function getTerminalURL(
  api: WorkspaceApiClient,
  workspaceID: string | undefined,
  repositoryID: string | undefined,
) {
  return workspaceID && repositoryID ? api.terminalURL(workspaceID, repositoryID) : undefined;
}

export function useChannelRepository(input: {
  api: WorkspaceApiClient;
  workspaceID: string | undefined;
  repositoryID: string | undefined;
  repositoryProvider: string | undefined;
  setTree: Dispatch<SetStateAction<WorkspaceTreeEntry[]>>;
  resetExpandedDirectories: () => void;
  setSelectedFile: (file: ApiRepositoryFile | undefined) => void;
  setReady: (ready: boolean) => void;
  setError: (error: string | undefined) => void;
}): void {
  const {
    api,
    workspaceID,
    repositoryID,
    repositoryProvider,
    setTree,
    resetExpandedDirectories,
    setSelectedFile,
    setReady,
    setError,
  } = input;
  useEffect(() => {
    setSelectedFile(undefined);
    resetExpandedDirectories();
    setReady(false);
    if (!workspaceID || !repositoryID) {
      setTree([]);
      return;
    }
    let active = true;
    const prepare = isLocalRepository(repositoryProvider)
      ? Promise.resolve()
      : api.prepareResource(workspaceID, repositoryID);
    void prepare
      .then(() => listRepositoryTree(api, workspaceID, repositoryID, repositoryProvider))
      .then((items) => {
        if (!active) return;
        setTree(items);
        setReady(true);
        setError(undefined);
      })
      .catch((error: unknown) => {
        if (!active) return;
        setTree([]);
        setError(
          error instanceof Error
            ? `Resource load failed: ${error.message}`
            : 'Resource load failed.',
        );
      });
    return () => {
      active = false;
    };
  }, [
    api,
    repositoryID,
    repositoryProvider,
    resetExpandedDirectories,
    setError,
    setReady,
    setSelectedFile,
    setTree,
    workspaceID,
  ]);
}

function isLocalRepository(provider: string | undefined): boolean {
  return provider === 'mirror' || provider === 'folder';
}

export function listRepositoryTree(
  api: WorkspaceApiClient,
  workspaceID: string,
  repositoryID: string,
  repositoryProvider: string | undefined,
  path = '',
): Promise<WorkspaceTreeEntry[]> {
  return isLocalRepository(repositoryProvider)
    ? listHostRepositoryTree(repositoryID, api.actorID, path)
    : api.listTree(workspaceID, repositoryID, path);
}

function readRepositoryFile(
  api: WorkspaceApiClient,
  workspaceID: string,
  repositoryID: string,
  repositoryProvider: string | undefined,
  path: string,
): Promise<ApiRepositoryFile> {
  return isLocalRepository(repositoryProvider)
    ? readHostRepositoryFile(repositoryID, api.actorID, path)
    : api.readFile(workspaceID, repositoryID, path);
}
