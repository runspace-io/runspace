'use client';

import { useEffect, useMemo, useState } from 'react';
import type { ApiGraphNode, WorkspaceApiClient } from '@/lib/api-client';
import type { AgentChatSelection } from './channel-agent-chats';

export type ChannelWorkItem = {
  node: ApiGraphNode;
  chat?: AgentChatSelection;
};

export function selectedWorkID(
  node: ApiGraphNode | undefined,
  chat: AgentChatSelection | undefined,
) {
  return node?.id ?? (chat?.registered ? `task:${chat.id}` : undefined);
}

export function useChannelSharedWork(
  api: WorkspaceApiClient,
  workspaceID: string | undefined,
  threadID: string | undefined,
  revision: number,
) {
  const [nodes, setNodes] = useState<ApiGraphNode[]>([]);
  useEffect(() => {
    let active = true;
    if (!workspaceID || !threadID) {
      setNodes([]);
      return;
    }
    void api
      .listGraphNodes(workspaceID, { threadID, limit: 100 })
      .then((items) => active && setNodes(items.filter(isSharedWork)))
      .catch(() => active && setNodes([]));
    return () => {
      active = false;
    };
  }, [api, revision, threadID, workspaceID]);
  return useMemo(() => nodes.map(toWorkItem), [nodes]);
}

function isSharedWork(node: ApiGraphNode) {
  return node.kind === 'task' || node.kind === 'artifact' || node.kind === 'action';
}

function toWorkItem(node: ApiGraphNode): ChannelWorkItem {
  if (node.kind !== 'task' || node.type !== 'agent_work') return { node };
  const entityID = metadataString(node, 'entity_id');
  const agentID = metadataString(node, 'agent_id');
  const resourceID = metadataString(node, 'resource_id');
  if (!entityID || !agentID || !resourceID) return { node };
  return {
    node,
    chat: {
      id: entityID,
      title: node.title,
      agentID,
      resourceID,
      registered: true,
    },
  };
}

function metadataString(node: ApiGraphNode, key: string) {
  const value = node.metadata?.[key];
  return typeof value === 'string' ? value : '';
}
