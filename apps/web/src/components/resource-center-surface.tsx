import type { ApiGraphNode, WorkspaceApiClient } from '@/lib/api-client';
import { GraphNodeSurface } from './graph-node-surface';
import { ResourceCenter } from './resource-center';

export function graphNodeSurface(
  api: WorkspaceApiClient,
  workspaceID: string | undefined,
  node: ApiGraphNode | undefined,
  onClose: () => void,
) {
  if (!node || !workspaceID) return undefined;
  return <GraphNodeSurface api={api} workspaceID={workspaceID} node={node} onClose={onClose} />;
}

export function resourceCenterSurface(
  api: WorkspaceApiClient,
  workspaceID: string | undefined,
  open: boolean,
  onOpen: (node: ApiGraphNode) => void,
  onClose: () => void,
) {
  if (!open || !workspaceID) return undefined;
  return <ResourceCenter api={api} workspaceID={workspaceID} onOpen={onOpen} onClose={onClose} />;
}
