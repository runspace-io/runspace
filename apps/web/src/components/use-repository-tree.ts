import { useCallback, useState } from 'react';
import { WorkspaceApiClient, type ApiRepositoryFile } from '@/lib/api-client';
import type { WorkspaceTreeEntry } from '@/lib/workspace-state';
import {
  getTerminalURL,
  listRepositoryTree,
  repositoryFileSelector,
  useChannelRepository,
} from './channel-repository';
import {
  collapsedDirectoryPaths,
  collapseRepositoryDirectory,
  expandRepositoryDirectory,
} from './repository-tree';

export function useRepositoryTree(input: {
  api: WorkspaceApiClient;
  workspaceID: string | undefined;
  repositoryID: string | undefined;
  repositoryProvider: string | undefined;
  setError: (error: string | undefined) => void;
}) {
  const { api, workspaceID, repositoryID, repositoryProvider, setError } = input;
  const [tree, setTree] = useState<WorkspaceTreeEntry[]>([]);
  const [expandedDirectories, setExpandedDirectories] = useState<string[]>([]);
  const [selectedFile, setSelectedFile] = useState<ApiRepositoryFile>();
  const [ready, setReady] = useState(false);
  const resetExpandedDirectories = useCallback(() => setExpandedDirectories([]), []);
  useChannelRepository({
    api,
    workspaceID,
    repositoryID,
    repositoryProvider,
    setTree,
    resetExpandedDirectories,
    setSelectedFile,
    setReady,
    setError,
  });
  const selectFile = repositoryFileSelector(api, workspaceID, repositoryID, repositoryProvider, {
    setFile: setSelectedFile,
    setError,
  });
  const toggleDirectory = (path: string) => {
    if (!workspaceID || !repositoryID) return;
    if (expandedDirectories.includes(path)) {
      setTree((current) => collapseRepositoryDirectory(current, path));
      setExpandedDirectories((current) => collapsedDirectoryPaths(current, path));
      return;
    }
    void listRepositoryTree(api, workspaceID, repositoryID, repositoryProvider, path)
      .then((children) => {
        setTree((current) => expandRepositoryDirectory(current, path, children));
        setExpandedDirectories((current) =>
          current.includes(path) ? current : [...current, path],
        );
      })
      .catch(() => setError('Unable to load directory.'));
  };
  return {
    tree,
    expandedDirectories,
    selectedFile,
    ready,
    terminalURL: getTerminalURL(api, workspaceID, repositoryID),
    selectFile,
    toggleDirectory,
  };
}
