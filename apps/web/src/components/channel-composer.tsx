'use client';

import { AtSign, Bold, Bot, Braces, Code2, Link2, Send, Sparkles } from 'lucide-react';
import { useEffect, useRef, useState, type ReactNode } from 'react';
import type { ApiGraphNode, ApiMember, WorkspaceApiClient } from '@/lib/api-client';
import { useMentionPicker } from './use-mention-picker';

export function ChannelComposer({
  api,
  workspaceID,
  draft,
  runAvailable = false,
  onDraftChange,
  onSend,
  onRunAgent,
}: {
  api: WorkspaceApiClient;
  workspaceID: string;
  draft: string;
  /** True when this channel has an agent and a resource to run it against. */
  runAvailable?: boolean;
  onDraftChange: (value: string) => void;
  onSend: () => void;
  onRunAgent?: (() => void) | undefined;
}) {
  const textarea = useRef<HTMLTextAreaElement>(null);
  const [resources, setResources] = useState<ApiGraphNode[]>([]);
  const [resourcePickerOpen, setResourcePickerOpen] = useState(false);
  useEffect(() => resize(textarea.current, draft), [draft]);
  const insert = (before: string, after = '') => {
    const element = textarea.current;
    if (!element) {
      onDraftChange(draft + before + after);
      return;
    }
    const start = element.selectionStart;
    const end = element.selectionEnd;
    const selected = draft.slice(start, end);
    onDraftChange(draft.slice(0, start) + before + selected + after + draft.slice(end));
    requestAnimationFrame(() => {
      element.focus();
      element.setSelectionRange(start + before.length, end + before.length);
    });
  };
  const mentions = useMentionPicker({
    api,
    workspaceID,
    draft,
    onDraftChange,
    textarea,
    insert,
  });
  const openResources = () => {
    mentions.close();
    setResourcePickerOpen(true);
    if (resources.length) return;
    void api
      .listGraphNodes(workspaceID, { kind: 'resource', limit: 12 })
      .then(setResources)
      .catch(() => setResources([]));
  };
  const insertResource = (resource: ApiGraphNode) => {
    const suffix = draft.endsWith('[[')
      ? `resource:${resource.id}]]`
      : `[[resource:${resource.id}]]`;
    insert(suffix);
    setResourcePickerOpen(false);
  };
  const updateDraft = (value: string, cursor: number) => {
    onDraftChange(value);
    if (mentions.updateQuery(value, cursor)) setResourcePickerOpen(false);
    if (value.slice(0, cursor).endsWith('[[')) openResources();
  };
  const openMentions = () => {
    setResourcePickerOpen(false);
    mentions.open();
  };
  return (
    <div className="composer-wrap">
      <section className="channel-composer" aria-label="Write a channel message">
        <div className="composer-toolbar" aria-label="Formatting tools">
          <ToolbarButton label="Bold" onClick={() => insert('**', '**')} icon={<Bold />} />
          <ToolbarButton label="Inline snippet" onClick={() => insert('`', '`')} icon={<Code2 />} />
          <ToolbarButton
            label="Fenced snippet"
            onClick={() => insert('\n```\n', '\n```\n')}
            icon={<Braces />}
          />
          <ToolbarButton label="Link" onClick={() => insert('[', '](https://)')} icon={<Link2 />} />
          <span />
          <ToolbarButton label="Mention" onClick={openMentions} icon={<AtSign />} />
          <ToolbarButton label="Insert Resource" onClick={openResources} icon={<Sparkles />} />
        </div>
        <textarea
          ref={textarea}
          value={draft}
          rows={1}
          aria-label="Message this channel"
          placeholder="Write with Markdown, @mention someone, or insert a Resource…"
          onChange={(event) =>
            updateDraft(
              event.target.value,
              event.target.selectionStart ?? event.target.value.length,
            )
          }
          onKeyDown={(event) => {
            if (mentions.handleKeyDown(event)) return;
            if (event.key === 'Enter' && !event.shiftKey) {
              event.preventDefault();
              onSend();
            }
          }}
        />
        <ComposerSuggestions
          open={resourcePickerOpen}
          resources={resources}
          onSelect={insertResource}
          onClose={() => setResourcePickerOpen(false)}
        />
        <MentionSuggestions
          open={mentions.query !== undefined}
          members={mentions.members}
          loaded={mentions.loaded}
          active={mentions.active}
          onSelect={mentions.select}
          onClose={mentions.close}
        />
        <ComposerFooter
          draft={draft}
          runAvailable={runAvailable}
          onSend={onSend}
          onRunAgent={onRunAgent}
        />
      </section>
    </div>
  );
}

function ComposerFooter({
  draft,
  runAvailable,
  onSend,
  onRunAgent,
}: {
  draft: string;
  runAvailable: boolean;
  onSend: () => void;
  onRunAgent?: (() => void) | undefined;
}) {
  return (
    <footer>
      <span>
        <kbd>Enter</kbd> send · <kbd>Shift Enter</kbd> newline · Markdown supported
      </span>
      <span className="composer-actions">
        {runAvailable && onRunAgent && (
          <button
            type="button"
            className="composer-run"
            disabled={!draft.trim()}
            onClick={onRunAgent}
            title="Run the channel agent against this resource in an isolated container"
          >
            <Bot size={14} />
            Run agent
          </button>
        )}
        <button
          className="composer-send"
          disabled={!draft.trim()}
          onClick={onSend}
          aria-label="Send message"
        >
          <Send size={15} />
        </button>
      </span>
    </footer>
  );
}

function MentionSuggestions({
  open,
  members,
  loaded,
  active,
  onSelect,
  onClose,
}: {
  open: boolean;
  members: ApiMember[];
  loaded: boolean;
  active: number;
  onSelect: (member: ApiMember) => void;
  onClose: () => void;
}) {
  if (!open) return null;
  return (
    <div className="composer-suggestions" role="listbox" aria-label="Mention a workspace member">
      <header>
        <strong>People in this workspace</strong>
        <button type="button" onClick={onClose}>
          Close
        </button>
      </header>
      {members.map((member, index) => (
        <button
          type="button"
          role="option"
          aria-selected={index === active}
          className={index === active ? 'is-active' : undefined}
          key={member.user_id}
          onMouseDown={(event) => event.preventDefault()}
          onClick={() => onSelect(member)}
        >
          <span className="mention-person">
            <span className="mention-avatar">{member.user_id.slice(0, 1).toUpperCase()}</span>
            <span>@{member.user_id}</span>
          </span>
          <small>{member.role}</small>
        </button>
      ))}
      {loaded && !members.length ? <p>No matching workspace members.</p> : null}
      {!loaded ? <p>Loading workspace members…</p> : null}
    </div>
  );
}

function ToolbarButton({
  label,
  icon,
  onClick,
}: {
  label: string;
  icon: ReactNode;
  onClick: () => void;
}) {
  return (
    <button type="button" title={label} aria-label={label} onClick={onClick}>
      {icon}
    </button>
  );
}

function ComposerSuggestions({
  open,
  resources,
  onSelect,
  onClose,
}: {
  open: boolean;
  resources: ApiGraphNode[];
  onSelect: (resource: ApiGraphNode) => void;
  onClose: () => void;
}) {
  if (!open) return null;
  return (
    <div className="composer-suggestions">
      <header>
        <strong>Insert workspace Resource</strong>
        <button onClick={onClose}>Close</button>
      </header>
      {resources.map((resource) => (
        <button key={resource.id} onClick={() => onSelect(resource)}>
          <span>{resource.title}</span>
          <small>{resource.type.replaceAll('_', ' ')}</small>
        </button>
      ))}
      {!resources.length ? <p>No shared Resources found.</p> : null}
    </div>
  );
}

function resize(element: HTMLTextAreaElement | null, value: string) {
  if (!element) return;
  element.style.height = '0px';
  element.style.height = `${Math.min(220, Math.max(44, element.scrollHeight))}px`;
  element.style.overflowY = element.scrollHeight > 220 ? 'auto' : 'hidden';
  element.dataset.empty = value ? 'false' : 'true';
}
