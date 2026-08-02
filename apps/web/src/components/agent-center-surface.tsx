import type { WorkspaceApiClient } from '@/lib/api-client';
import { AgentCenter } from './agent-center';

export function agentCenterSurface(
  api: WorkspaceApiClient,
  workspaceID: string | undefined,
  open: boolean,
  onClose: () => void,
) {
  if (!open || !workspaceID) return undefined;
  return <AgentCenter api={api} workspaceID={workspaceID} onClose={onClose} />;
}
