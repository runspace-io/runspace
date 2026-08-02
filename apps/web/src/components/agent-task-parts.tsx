import { Bot, Check, Send, Share2, Square, Users, X } from 'lucide-react';
import type { LocalAgentSession, LocalTaskMessage } from '@/lib/host-agent-client';
import type { RepositorySummary } from '@/lib/workspace-state';

export function AgentTaskHeader({
  title,
  status,
  accessOpen,
  accessAvailable,
  onAccessChange,
  onClose,
  closeLabel = 'Close agent chat',
}: {
  title: string;
  status: string;
  accessOpen: boolean;
  accessAvailable: boolean;
  onAccessChange: () => void;
  onClose: () => void;
  closeLabel?: string;
}) {
  return (
    <header className="agent-task-header">
      <div className="agent-task-identity">
        <span className="agent-task-mark">
          <Bot size={17} />
        </span>
        <div>
          <p className="eyebrow">LOCAL AGENT CHAT / PRIVATE BY DEFAULT</p>
          <h2 id="agent-task-title">{title || 'New agent chat'}</h2>
        </div>
      </div>
      <div className="agent-task-header-actions">
        <TaskStatus status={status} />
        {accessAvailable && (
          <button
            className="agent-task-access-trigger"
            type="button"
            aria-expanded={accessOpen}
            onClick={onAccessChange}
          >
            <Users size={14} />
            Access
          </button>
        )}
        <button
          className="icon-button"
          aria-label={closeLabel}
          title={closeLabel}
          onClick={onClose}
        >
          <X size={17} />
        </button>
      </div>
    </header>
  );
}

export function AgentTaskMeta({
  resources,
  resourceID,
  provider,
  busy,
  onResourceChange,
}: {
  resources: readonly RepositorySummary[];
  resourceID: string;
  provider?: string | undefined;
  busy: boolean;
  onResourceChange: (id: string) => void;
}) {
  return (
    <div className="agent-task-meta">
      <label>
        <span>Resource</span>
        <select
          aria-label="Task resource"
          value={resourceID}
          disabled={busy}
          onChange={(event) => onResourceChange(event.target.value)}
        >
          {resources.map((resource) => (
            <option key={resource.id} value={resource.id}>
              {resource.fullName}
            </option>
          ))}
        </select>
      </label>
      <div>
        <span>Execution</span>
        <strong>Owner device · {provider ?? 'No resource'}</strong>
      </div>
      <div>
        <span>Channel visibility</span>
        <strong>Private · share explicitly</strong>
      </div>
    </div>
  );
}

export function TaskLog({
  messages,
  shared,
  onShare,
}: {
  messages: readonly LocalTaskMessage[];
  shared: ReadonlySet<string>;
  onShare: (message: LocalTaskMessage) => void;
}) {
  if (messages.length === 0) return <TaskEmptyState />;
  return (
    <ol className="agent-task-log" aria-label="Private agent chat">
      {messages.map((message, index) => (
        <li key={message.id} className={logEntryClass(message)}>
          <div className="agent-task-log-index">{String(index + 1).padStart(2, '0')}</div>
          <div className="agent-task-log-body">
            <header>
              <strong>{entryLabel(message)}</strong>
              <time>{formatTime(message.created_at)}</time>
            </header>
            {/* A command the agent ran is terminal output, so keep its
                whitespace instead of reflowing it as prose. */}
            {isToolCall(message) ? <pre>{message.body}</pre> : <p>{message.body}</p>}
          </div>
          {message.role === 'agent' && !isToolCall(message) && (
            <button
              type="button"
              className="agent-task-share"
              disabled={shared.has(message.id)}
              onClick={() => onShare(message)}
            >
              {shared.has(message.id) ? <Check size={13} /> : <Share2 size={13} />}
              {shared.has(message.id) ? 'Shared' : 'Share'}
            </button>
          )}
        </li>
      ))}
    </ol>
  );
}

export function AgentTaskComposer({
  session,
  instruction,
  resourceID,
  busy,
  onInstructionChange,
  onRun,
  onCancel,
}: {
  session?: LocalAgentSession | undefined;
  instruction: string;
  resourceID: string;
  busy: boolean;
  onInstructionChange: (value: string) => void;
  onRun: () => void;
  onCancel: () => void;
}) {
  return (
    <footer className="agent-task-composer">
      <label htmlFor="task-instruction">Instruction</label>
      <textarea
        id="task-instruction"
        value={instruction}
        disabled={!resourceID || busy}
        placeholder={
          session?.messages.length
            ? 'Continue this agent chat…'
            : 'Describe what you want the agent to work on…'
        }
        onChange={(event) => onInstructionChange(event.target.value)}
        onKeyDown={(event) => {
          if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') onRun();
        }}
      />
      <div>
        <span>Private input · Ctrl/⌘ + Enter to run</span>
        <div>
          {busy && (
            <button className="dialog-secondary" type="button" onClick={onCancel}>
              <Square size={13} />
              Cancel
            </button>
          )}
          <button
            className="dialog-primary"
            type="button"
            disabled={!instruction.trim() || !resourceID || busy}
            onClick={onRun}
          >
            <Send size={13} />
            {session?.messages.length ? 'Send instruction' : 'Start chat'}
          </button>
        </div>
      </div>
    </footer>
  );
}

function isToolCall(message: LocalTaskMessage): boolean {
  return message.kind === 'tool_call';
}

function logEntryClass(message: LocalTaskMessage): string {
  return isToolCall(message) ? 'is-agent is-tool-call' : `is-${message.role}`;
}

function entryLabel(message: LocalTaskMessage): string {
  if (isToolCall(message)) return 'Agent terminal';
  return message.role === 'agent' ? 'Agent response' : 'Your instruction';
}

function TaskStatus({ status }: { status: string }) {
  return (
    <span className={`agent-task-status is-${status}`}>
      <i />
      {status.replace('_', ' ')}
    </span>
  );
}

function TaskEmptyState() {
  return (
    <div className="agent-task-empty">
      <span>PRIVATE AGENT CHAT</span>
      <strong>Start work on your own device.</strong>
      <p>Nothing appears in the channel until you share the chat or an individual result.</p>
    </div>
  );
}

function formatTime(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.valueOf())
    ? 'now'
    : date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}
