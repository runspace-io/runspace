'use client';

import {
  Bot,
  Braces,
  MessageSquareText,
  Sparkles,
  SquareTerminal,
  type LucideIcon,
} from 'lucide-react';
import { useEffect, useLayoutEffect, useRef, type CSSProperties } from 'react';
import type { TimelineItem } from '@/lib/workspace-state';
import type { ApiGraphNode, WorkspaceApiClient } from '@/lib/api-client';
import { RichMessageBody } from './rich-message-body';

export function Timeline({
  items,
  api,
  workspaceID,
  onOpenNode,
}: {
  items: readonly TimelineItem[];
  api: WorkspaceApiClient;
  workspaceID: string;
  onOpenNode: (node: ApiGraphNode) => void;
}) {
  const feed = useRef<HTMLDivElement>(null);
  const content = useRef<HTMLDivElement>(null);
  const pinnedToLatest = useRef(true);
  const initialized = useRef(false);
  const latestMessageID = items.at(-1)?.id;
  useLayoutEffect(() => {
    scrollToLatest(feed.current, initialized.current);
    initialized.current = true;
    pinnedToLatest.current = true;
  }, [latestMessageID]);
  useEffect(() => {
    const element = content.current;
    if (!element || typeof ResizeObserver === 'undefined') return;
    const observer = new ResizeObserver(() => {
      if (pinnedToLatest.current) scrollToLatest(feed.current, false);
    });
    observer.observe(element);
    return () => observer.disconnect();
  }, []);
  return (
    <div
      ref={feed}
      className="timeline message-feed"
      aria-live="polite"
      onScroll={(event) => {
        pinnedToLatest.current = isNearLatest(event.currentTarget);
      }}
    >
      <div ref={content} className="message-feed-content">
        {items.length === 0 && <TimelineEmptyState />}
        {items.map((item, index) => {
          const continuation = isContinuation(item, items[index - 1]);
          return (
            <TimelineMessage
              key={item.id}
              item={item}
              continuation={continuation}
              api={api}
              workspaceID={workspaceID}
              onOpenNode={onOpenNode}
            />
          );
        })}
      </div>
    </div>
  );
}

function TimelineEmptyState() {
  return (
    <div className="timeline-empty-state">
      <MessageSquareText size={22} aria-hidden="true" />
      <strong>No messages yet</strong>
      <p>Say hello, or connect an agent and ask it to get started.</p>
    </div>
  );
}

function scrollToLatest(element: HTMLDivElement | null, animate: boolean) {
  if (!element) return;
  element.scrollTo({
    top: element.scrollHeight,
    behavior: animate && !reducedMotion() ? 'smooth' : 'auto',
  });
}

function isNearLatest(element: HTMLDivElement): boolean {
  return element.scrollHeight - element.scrollTop - element.clientHeight < 72;
}

function reducedMotion(): boolean {
  return window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false;
}

function TimelineMessage({
  item,
  continuation,
  api,
  workspaceID,
  onOpenNode,
}: {
  item: TimelineItem;
  continuation: boolean;
  api: WorkspaceApiClient;
  workspaceID: string;
  onOpenNode: (node: ApiGraphNode) => void;
}) {
  const classes = [
    'message-row',
    `message-row--${item.role}`,
    item.activity && 'is-activity',
    continuation && 'is-continuation',
  ]
    .filter(Boolean)
    .join(' ');
  return (
    <article className={classes}>
      {continuation ? (
        <time className="message-continuation-time">{item.time}</time>
      ) : (
        <MessageAvatar item={item} />
      )}
      <div className="message-row-content">
        {!continuation ? <MessageHeader item={item} /> : null}
        <RichMessageBody
          api={api}
          workspaceID={workspaceID}
          body={item.body}
          onOpenNode={onOpenNode}
        />
      </div>
    </article>
  );
}

function MessageHeader({ item }: { item: TimelineItem }) {
  return (
    <header className="message-row-header">
      <strong>{item.author}</strong>
      {item.provider ? (
        <span className="message-provider">
          <ProviderIcon providerID={item.providerID} />
          {item.provider}
        </span>
      ) : null}
      <time>{item.time}</time>
    </header>
  );
}

function isContinuation(item: TimelineItem, previous: TimelineItem | undefined): boolean {
  if (item.activity || previous?.activity) return false;
  return previous?.author === item.author && previous?.provider === item.provider;
}

function MessageAvatar({ item }: { item: TimelineItem }) {
  if (item.provider) {
    return (
      <span className="message-avatar message-avatar--agent" aria-label={`${item.provider} agent`}>
        <ProviderIcon providerID={item.providerID} />
      </span>
    );
  }
  const initials = item.author === 'You' ? 'Y' : initialsFor(item.author);
  const style = { '--avatar-hue': avatarHue(item.author) } as CSSProperties;
  return (
    <span className="message-avatar" aria-hidden="true" style={style}>
      {initials}
    </span>
  );
}

/** A stable, per-author hue so avatars read as distinct people, not identical gray boxes. */
function avatarHue(name: string): number {
  let hash = 0;
  for (const character of name) hash = (hash * 31 + character.charCodeAt(0)) % 360;
  return hash;
}

function ProviderIcon({ providerID }: { providerID: string | undefined }) {
  const Icon = providerIcon(providerID);
  return <Icon aria-hidden="true" />;
}

function providerIcon(providerID: string | undefined): LucideIcon {
  const provider = providerID?.toLocaleLowerCase() ?? '';
  if (provider.includes('codex')) return SquareTerminal;
  if (provider.includes('claude')) return Sparkles;
  if (provider.includes('opencode')) return Braces;
  return Bot;
}

function initialsFor(name: string): string {
  return name
    .split(/\s+/)
    .slice(0, 2)
    .map((part) => part.charAt(0))
    .join('')
    .toLocaleUpperCase();
}
