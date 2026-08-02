import { describe, expect, it, vi } from 'vitest';
import type { ApiAgentTask, WorkspaceApiClient } from '@/lib/api-client';
import { loadChannelTask } from './agent-task-remote';

function task(overrides: Partial<ApiAgentTask> = {}): ApiAgentTask {
  return {
    id: 'local_session_1',
    workspace_id: 'workspace-1',
    thread_id: 'thread-1',
    owner_id: 'admin',
    agent_id: 'local_agent_abc',
    resource_id: 'repo-1',
    title: 'Investigate the failing test',
    status: 'waiting_approval',
    created_at: '2026-08-01T10:00:00Z',
    updated_at: '2026-08-01T10:01:00Z',
    ...overrides,
  };
}

function client(overrides: Partial<WorkspaceApiClient>) {
  return { actorID: 'nahid', ...overrides } as unknown as WorkspaceApiClient;
}

const request = {
  workspaceID: 'workspace-1',
  threadID: 'thread-1',
  agentID: 'local_agent_abc',
  resourceID: 'repo-1',
  taskID: 'local_session_1',
  registered: true,
};

describe('loadChannelTask for someone else’s task', () => {
  // The transcript lives on the owner's device, so a grantee has to read the
  // server copy. Returning an empty list here left them intervening blind.
  it('hydrates the transcript from the gateway', async () => {
    const listAgentTaskMessages = vi.fn().mockResolvedValue([
      { id: 'm1', role: 'user', body: 'clean the build', created_at: '2026-08-01T10:00:00Z' },
      { id: 'm2', role: 'agent', body: 'Reading main.go', created_at: '2026-08-01T10:00:05Z' },
    ]);
    const api = client({
      listAgentTasks: vi.fn().mockResolvedValue([task()]),
      listAgentTaskMessages,
    });

    const loaded = await loadChannelTask({ api, ...request });

    expect(listAgentTaskMessages).toHaveBeenCalledWith('local_session_1');
    expect(loaded.remoteTask?.id).toBe('local_session_1');
    expect(loaded.session.messages.map((message) => message.body)).toEqual([
      'clean the build',
      'Reading main.go',
    ]);
    expect(loaded.session.status).toBe('waiting_approval');
  });

  // Losing the transcript must not blank the task; the viewer still needs its
  // identity and status to understand why the agent is stopped.
  it('still opens the task when the transcript cannot be read', async () => {
    const api = client({
      listAgentTasks: vi.fn().mockResolvedValue([task()]),
      listAgentTaskMessages: vi.fn().mockRejectedValue(new Error('forbidden')),
    });

    const loaded = await loadChannelTask({ api, ...request });

    expect(loaded.session.messages).toEqual([]);
    expect(loaded.title).toBe('Investigate the failing test');
  });

  // The owner's own chat is authoritative on their device, not on the server.
  it('does not fetch the server transcript for the caller’s own task', async () => {
    const listAgentTaskMessages = vi.fn();
    const api = client({
      listAgentTasks: vi.fn().mockResolvedValue([task({ owner_id: 'nahid' })]),
      listAgentTaskMessages,
    });

    // The local path reaches the Host Agent, which is absent here; the point of
    // the test is only that the server transcript was never consulted.
    await loadChannelTask({ api, ...request }).catch(() => undefined);

    expect(listAgentTaskMessages).not.toHaveBeenCalled();
  });
});
