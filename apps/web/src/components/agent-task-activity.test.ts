import { describe, expect, it } from 'vitest';
import type { LocalAgentSession } from '@/lib/host-agent-client';
import { deriveAgentActivity } from './agent-task-activity';

function session(messages: LocalAgentSession['messages']): LocalAgentSession {
  return {
    id: 'local_session_1',
    title: 'Fix the failing test',
    agent_id: 'local_agent_abc',
    resource_id: 'repo-1',
    thread_id: 'thread-1',
    status: 'running',
    pause_support: 'cancel-only',
    messages,
  };
}

describe('deriveAgentActivity', () => {
  it('reports nothing when the turn is not running', () => {
    expect(deriveAgentActivity(false, session([]), 0)).toBeUndefined();
  });

  it('reports thinking when no message has arrived since the turn started', () => {
    const current = session([{ id: 'm1', role: 'user', body: 'fix it', created_at: 'now' }]);
    expect(deriveAgentActivity(true, current, 1)).toEqual({ kind: 'thinking' });
  });

  it('reports the running tool once a tool_call chunk streams in', () => {
    const current = session([
      { id: 'm1', role: 'user', body: 'fix it', created_at: 'now' },
      { id: 'm2', role: 'agent', kind: 'tool_call', body: 'npm test\n(output)', created_at: 'now' },
    ]);
    expect(deriveAgentActivity(true, current, 1)).toEqual({ kind: 'tool', command: 'npm test' });
  });

  it('reports nothing extra once prose is streaming — the transcript is the indicator', () => {
    const current = session([
      { id: 'm1', role: 'user', body: 'fix it', created_at: 'now' },
      { id: 'm2', role: 'agent', body: 'Looking into it now.', created_at: 'now' },
    ]);
    expect(deriveAgentActivity(true, current, 1)).toBeUndefined();
  });
});
