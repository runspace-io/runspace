import type { WorkspaceApiClient } from '@/lib/api-client';
import { validateWorkspaceForm, type WorkspaceSummary } from '@/lib/workspace-state';

export function createWorkspaceRequest(input: {
  api: WorkspaceApiClient;
  name: string;
  setWorkspaces: (update: (items: WorkspaceSummary[]) => WorkspaceSummary[]) => void;
  setActiveWorkspace: (workspace: WorkspaceSummary) => void;
  setWorkspaceName: (name: string) => void;
  setDialogOpen: (open: boolean) => void;
  setError: (error: string | undefined) => void;
}) {
  const {
    api,
    name,
    setWorkspaces,
    setActiveWorkspace,
    setWorkspaceName,
    setDialogOpen,
    setError,
  } = input;
  const error = validateWorkspaceForm({ name });
  if (error) return setError(error);
  void api
    .createWorkspace(name.trim())
    .then((workspace) => {
      setWorkspaces((current) => [...current, workspace]);
      setActiveWorkspace(workspace);
      setWorkspaceName('');
      setDialogOpen(false);
    })
    .catch(() => setError('Unable to create workspace. Please retry.'));
}
