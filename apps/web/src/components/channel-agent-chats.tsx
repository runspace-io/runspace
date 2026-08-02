'use client';

import { useEffect, useState } from 'react';
import { MessageSquare, Share2 } from 'lucide-react';
import type { ApiAgentTask, WorkspaceApiClient } from '@/lib/api-client';
import { listLocalAgentChats, type LocalAgentChatSummary } from '@/lib/host-agent-client';
import { DisclosureSection } from './disclosure-section';

export type AgentChatSelection = {
  id: string;
  title: string;
  agentID: string;
  resourceID: string;
  registered: boolean;
};

export function ChannelAgentChats({
  api,
  workspaceID,
  threadID,
  revision,
  onOpen,
  onShared,
}: {
  api: WorkspaceApiClient;
  workspaceID: string;
  threadID: string;
  revision: number;
  onOpen: (chat: AgentChatSelection) => void;
  onShared: (chat: AgentChatSelection) => void;
}) {
  const catalog = useAgentChatCatalog(api, workspaceID, threadID, revision);
  const share = async (chat: LocalAgentChatSummary) => {
    const now = new Date().toISOString();
    const task = await api.upsertAgentTask({
      id: chat.id,
      workspace_id: workspaceID,
      thread_id: threadID,
      owner_id: api.actorID,
      agent_id: chat.agent_id,
      resource_id: chat.resource_id,
      title: chat.title,
      status: chat.status === 'draft' ? 'ready' : chat.status,
      created_at: now,
      updated_at: now,
    });
    const selection: AgentChatSelection = {
      id: task.id,
      title: task.title,
      agentID: task.agent_id,
      resourceID: task.resource_id,
      registered: true,
    };
    catalog.refresh();
    onShared(selection);
  };
  if (catalog.loading) {
    return <p className="agent-chat-catalog-loading">Reading chat history…</p>;
  }
  return (
    <DisclosureSection
      label="Chats"
      summary={chatHistorySummary(catalog.shared.length, catalog.local.length)}
      defaultOpen={catalog.shared.length > 0 || catalog.local.length > 0}
    >
      <div className="agent-chat-catalog">
        <ChatGroup
          label="Shared with this channel"
          empty="No agent sessions shared yet."
          chats={catalog.shared.map(sharedSelection)}
          onOpen={onOpen}
        />
        <ChatGroup
          label="My local chats"
          empty="No private chats in this workspace."
          chats={catalog.local.map(localSelection)}
          onOpen={onOpen}
          onShare={(chat) => {
            const local = catalog.local.find((item) => item.id === chat.id);
            if (local) void share(local).catch(catalog.setError);
          }}
        />
        {catalog.error && <p className="agent-chat-catalog-error">{catalog.error.message}</p>}
      </div>
    </DisclosureSection>
  );
}

export function chatHistorySummary(sharedCount: number, localCount: number): string {
  if (sharedCount === 0 && localCount === 0) return 'No chats yet';
  const parts: string[] = [];
  if (sharedCount > 0) parts.push(`${sharedCount} shared`);
  if (localCount > 0) parts.push(`${localCount} private`);
  return parts.join(' · ');
}

function ChatGroup({
  label,
  empty,
  chats,
  onOpen,
  onShare,
}: {
  label: string;
  empty: string;
  chats: AgentChatSelection[];
  onOpen: (chat: AgentChatSelection) => void;
  onShare?: ((chat: AgentChatSelection) => void) | undefined;
}) {
  return (
    <section className="agent-chat-group">
      <header>
        <span>{label}</span>
        <b>{chats.length}</b>
      </header>
      {chats.length === 0 ? (
        <p>{empty}</p>
      ) : (
        <div>
          {chats.map((chat) => (
            <div className="agent-chat-row" key={chat.id}>
              <button type="button" onClick={() => onOpen(chat)}>
                <MessageSquare size={13} />
                <span>
                  <strong>{chat.title}</strong>
                  <small>{chat.registered ? 'Channel access' : 'Private on this device'}</small>
                </span>
              </button>
              {onShare ? (
                <button
                  type="button"
                  className="agent-chat-share-button"
                  aria-label={`Share ${chat.title} with channel`}
                  onClick={() => onShare(chat)}
                >
                  <Share2 size={12} />
                </button>
              ) : null}
            </div>
          ))}
        </div>
      )}
    </section>
  );
}

function useAgentChatCatalog(
  api: WorkspaceApiClient,
  workspaceID: string,
  threadID: string,
  revision: number,
) {
  const [local, setLocal] = useState<LocalAgentChatSummary[]>([]);
  const [shared, setShared] = useState<ApiAgentTask[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error>();
  const [refreshKey, setRefreshKey] = useState(0);
  useEffect(() => {
    let active = true;
    setLoading(true);
    void Promise.all([
      listLocalAgentChats(api.actorID, workspaceID),
      api.listAgentTasks(workspaceID, threadID),
    ])
      .then(([localItems, sharedItems]) => {
        if (!active) return;
        const sharedIDs = new Set(sharedItems.map((item) => item.id));
        setLocal(localItems.filter((item) => !sharedIDs.has(item.id)));
        setShared(sharedItems);
        setError(undefined);
      })
      .catch((reason) => active && setError(asError(reason)))
      .finally(() => active && setLoading(false));
    return () => {
      active = false;
    };
  }, [api, refreshKey, revision, threadID, workspaceID]);
  return {
    local,
    shared,
    loading,
    error,
    setError: (reason: unknown) => setError(asError(reason)),
    refresh: () => setRefreshKey((value) => value + 1),
  };
}

function localSelection(chat: LocalAgentChatSummary): AgentChatSelection {
  return {
    id: chat.id,
    title: chat.title,
    agentID: chat.agent_id,
    resourceID: chat.resource_id,
    registered: false,
  };
}

function sharedSelection(chat: ApiAgentTask): AgentChatSelection {
  return {
    id: chat.id,
    title: chat.title,
    agentID: chat.agent_id,
    resourceID: chat.resource_id,
    registered: true,
  };
}

function asError(reason: unknown) {
  return reason instanceof Error ? reason : new Error('Could not load agent chats.');
}
