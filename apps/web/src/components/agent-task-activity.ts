import type { Dispatch, SetStateAction } from 'react';
import { getLocalAgentSession, type LocalAgentSession } from '@/lib/host-agent-client';
import type { AgentTaskProps } from './agent-task-controller';

const POLL_INTERVAL_MS = 900;

/**
 * Polls the Host Agent's session while a turn is in flight so tool calls and
 * streamed replies appear as they happen instead of all at once when the
 * prompt call finally resolves. Safe to poll concurrently with the in-flight
 * prompt: the session's per-turn lock only serializes prompts against each
 * other, the read path here only ever takes the brief server-config lock.
 */
export function pollSessionWhileRunning(
  props: AgentTaskProps,
  resourceID: string,
  taskID: string,
  setSession: Dispatch<SetStateAction<LocalAgentSession | undefined>>,
): () => void {
  let active = true;
  let timer: ReturnType<typeof setTimeout> | undefined;
  const tick = () => {
    if (!active) return;
    void getLocalAgentSession({
      userID: props.api.actorID,
      agentID: props.agentID,
      resourceID,
      threadID: props.threadID,
      taskID,
    })
      .then((latest) => {
        if (active) setSession(latest);
      })
      .catch(() => {
        // A missed poll just waits for the next tick or the final await.
      })
      .finally(() => {
        if (active) timer = setTimeout(tick, POLL_INTERVAL_MS);
      });
  };
  timer = setTimeout(tick, POLL_INTERVAL_MS);
  return () => {
    active = false;
    clearTimeout(timer);
  };
}

export type AgentActivity = { kind: 'thinking' } | { kind: 'tool'; command: string } | undefined;

/**
 * Derives a live status from whatever has streamed in since a turn started,
 * limited to the three states safe to promise across every agent adapter:
 * thinking (nothing back yet), running a tool (already-distinguished ACP
 * update kind), or streaming prose (the transcript itself is the indicator,
 * so there is nothing extra to render for that case).
 */
export function deriveAgentActivity(
  busy: boolean,
  session: LocalAgentSession | undefined,
  messageCountAtStart: number,
): AgentActivity {
  if (!busy) return undefined;
  const messages = session?.messages ?? [];
  if (messages.length <= messageCountAtStart) return { kind: 'thinking' };
  const latest = messages.at(-1);
  if (latest?.kind === 'tool_call') {
    return { kind: 'tool', command: latest.body.split('\n', 1)[0] ?? '' };
  }
  return undefined;
}
