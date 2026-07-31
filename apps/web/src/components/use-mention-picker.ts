import { useMemo, useState, type KeyboardEvent, type RefObject } from 'react';
import type { ApiMember, WorkspaceApiClient } from '@/lib/api-client';

export function useMentionPicker({
  api,
  workspaceID,
  draft,
  onDraftChange,
  textarea,
  insert,
}: {
  api: WorkspaceApiClient;
  workspaceID: string;
  draft: string;
  onDraftChange: (value: string) => void;
  textarea: RefObject<HTMLTextAreaElement | null>;
  insert: (before: string, after?: string) => void;
}) {
  const [allMembers, setAllMembers] = useState<ApiMember[]>([]);
  const [loadedWorkspaceID, setLoadedWorkspaceID] = useState<string>();
  const [loadingWorkspaceID, setLoadingWorkspaceID] = useState<string>();
  const [query, setQuery] = useState<string>();
  const [active, setActive] = useState(0);
  const members = useMemo(
    () => filterMembers(loadedWorkspaceID === workspaceID ? allMembers : [], query),
    [allMembers, loadedWorkspaceID, query, workspaceID],
  );
  const load = () => {
    if (loadedWorkspaceID === workspaceID || loadingWorkspaceID === workspaceID) return;
    setLoadingWorkspaceID(workspaceID);
    void api
      .listMembers(workspaceID)
      .then((items) => {
        setAllMembers(items);
        setLoadedWorkspaceID(workspaceID);
      })
      .catch(() => {
        setAllMembers([]);
        setLoadedWorkspaceID(workspaceID);
      })
      .finally(() => setLoadingWorkspaceID(undefined));
  };
  const close = () => setQuery(undefined);
  const open = () => {
    load();
    insert('@');
    setQuery('');
    setActive(0);
  };
  const updateQuery = (value: string, cursor: number) => {
    const mention = findMention(value, cursor);
    setQuery(mention?.query);
    if (!mention) return false;
    load();
    setActive(0);
    return true;
  };
  const select = (member: ApiMember) => {
    const element = textarea.current;
    const cursor = element?.selectionStart ?? draft.length;
    const start = findMention(draft, cursor)?.start ?? cursor;
    const text = `@${member.user_id} `;
    onDraftChange(draft.slice(0, start) + text + draft.slice(cursor));
    close();
    requestAnimationFrame(() => {
      element?.focus();
      element?.setSelectionRange(start + text.length, start + text.length);
    });
  };
  const handleKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (query === undefined) return false;
    if (event.key === 'Escape') {
      event.preventDefault();
      close();
      return true;
    }
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault();
      moveSelection(event.key, members.length, setActive);
      return true;
    }
    if (event.key !== 'Enter' && event.key !== 'Tab') return false;
    const member = members[active] ?? members[0];
    if (!member) return false;
    event.preventDefault();
    select(member);
    return true;
  };
  return {
    active,
    close,
    handleKeyDown,
    loaded: loadedWorkspaceID === workspaceID,
    members,
    open,
    query,
    select,
    updateQuery,
  };
}

export function findMention(value: string, cursor: number) {
  const beforeCursor = value.slice(0, cursor);
  const match = /(^|[\s([{])@([\w.-]*)$/.exec(beforeCursor);
  if (!match) return undefined;
  const query = match[2] ?? '';
  return {
    start: beforeCursor.length - query.length - 1,
    query,
  };
}

function filterMembers(members: ApiMember[], query: string | undefined): ApiMember[] {
  if (query === undefined) return [];
  const normalized = query.toLowerCase();
  return members
    .filter((member) => member.user_id.toLowerCase().includes(normalized))
    .sort((left, right) => {
      const leftStarts = left.user_id.toLowerCase().startsWith(normalized);
      const rightStarts = right.user_id.toLowerCase().startsWith(normalized);
      return Number(rightStarts) - Number(leftStarts) || left.user_id.localeCompare(right.user_id);
    })
    .slice(0, 8);
}

function moveSelection(
  key: string,
  count: number,
  setActive: (update: (current: number) => number) => void,
) {
  const direction = key === 'ArrowDown' ? 1 : -1;
  setActive((current) => (count ? (current + direction + count) % count : 0));
}
