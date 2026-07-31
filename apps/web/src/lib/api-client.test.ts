import { describe, expect, it, vi } from 'vitest';
import { ApiError, eventToTimelineItem, WorkspaceApiClient } from './api-client';
import { eventRunContext } from './api-normalizers';

describe('WorkspaceApiClient', () => {
  it('retries transient responses and sends the identity header', async () => {
    const fetcher = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(new Response('temporarily unavailable', { status: 503 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ workspaces: [] }), { status: 200 }));
    const client = new WorkspaceApiClient({ fetcher, sleep: vi.fn().mockResolvedValue(undefined) });

    await expect(client.listWorkspaces()).resolves.toEqual([]);
    expect(fetcher).toHaveBeenCalledTimes(2);
    expect(fetcher.mock.calls[0]?.[1]?.headers).toMatchObject({ 'x-user-id': 'admin' });
  });

  it('does not retry permanent client errors', async () => {
    const fetcher = vi
      .fn<typeof fetch>()
      .mockResolvedValue(new Response('forbidden', { status: 403 }));
    const client = new WorkspaceApiClient({ fetcher, sleep: vi.fn() });

    await expect(client.listWorkspaces()).rejects.toEqual(new ApiError(403, 'forbidden'));
    expect(fetcher).toHaveBeenCalledTimes(1);
  });

  it('surfaces structured API errors as readable messages', async () => {
    const fetcher = vi
      .fn<typeof fetch>()
      .mockResolvedValue(
        new Response(JSON.stringify({ error: 'repository is not connected' }), { status: 400 }),
      );
    const client = new WorkspaceApiClient({ fetcher });

    await expect(client.listWorkspaces()).rejects.toEqual(
      new ApiError(400, 'repository is not connected'),
    );
  });

  it('normalizes an empty message response from the API', async () => {
    const fetcher = vi
      .fn<typeof fetch>()
      .mockResolvedValue(new Response(JSON.stringify({ messages: null }), { status: 200 }));
    const client = new WorkspaceApiClient({ fetcher });

    await expect(client.listMessages('workspace-1', 'thread-1')).resolves.toEqual([]);
  });

  it('creates and normalizes an agent run', async () => {
    const fetcher = vi.fn<typeof fetch>().mockResolvedValue(
      new Response(
        JSON.stringify({
          ID: 'run-1',
          WorkspaceID: 'workspace-1',
          ThreadID: 'thread-1',
          ChannelID: 'channel-1',
          Prompt: 'Fix it',
          Status: 'queued',
        }),
        { status: 202 },
      ),
    );
    const client = new WorkspaceApiClient({ fetcher });

    await expect(
      client.createRun('thread-1', {
        runID: 'run-1',
        workspaceID: 'workspace-1',
        repositoryID: 'repo-1',
        prompt: 'Fix it',
      }),
    ).resolves.toMatchObject({
      id: 'run-1',
      thread_id: 'thread-1',
      channel_id: 'channel-1',
      status: 'queued',
    });
    expect(fetcher).toHaveBeenCalledWith(
      expect.stringContaining('/threads/thread-1/runs'),
      expect.objectContaining({ method: 'POST' }),
    );
  });
});

describe('WorkspaceApiClient resource and history APIs', () => {
  it('uses resource contracts for channel connections', async () => {
    const fetcher = vi.fn<typeof fetch>().mockResolvedValue(
      new Response(
        JSON.stringify({
          id: 'channel-1',
          workspace_id: 'workspace-1',
          name: 'general',
          resource_ids: ['resource-1'],
        }),
        { status: 201 },
      ),
    );
    const client = new WorkspaceApiClient({ fetcher });

    await expect(
      client.createChannel('workspace-1', 'general', '', {
        repositoryIDs: ['resource-1'],
      }),
    ).resolves.toMatchObject({ repository_ids: ['resource-1'] });
    const body = String(fetcher.mock.calls[0]?.[1]?.body);
    expect(body).toContain('"resource_ids":["resource-1"]');
    expect(body).not.toContain('"repository_ids"');
  });

  it('prepares a connected Git resource before reading its tree', async () => {
    const fetcher = vi
      .fn<typeof fetch>()
      .mockResolvedValue(
        new Response(JSON.stringify({ path: '/workspace/repo', ref: 'main' }), { status: 202 }),
      );
    const client = new WorkspaceApiClient({ fetcher });
    await expect(client.prepareResource('workspace-1', 'repo-1')).resolves.toEqual({
      path: '/workspace/repo',
      ref: 'main',
    });
    expect(fetcher).toHaveBeenCalledWith(
      expect.stringContaining('/workspaces/workspace-1/resources/repo-1/clone'),
      expect.objectContaining({ method: 'POST' }),
    );
  });

  it('omits ignored repository metadata from the file tree', async () => {
    const fetcher = vi.fn<typeof fetch>().mockResolvedValue(
      new Response(
        JSON.stringify({
          entries: [
            { path: '.git', kind: 'directory', ignored: true },
            { path: 'src', kind: 'directory', ignored: false },
          ],
        }),
        { status: 200 },
      ),
    );
    const client = new WorkspaceApiClient({ fetcher });

    await expect(client.listTree('workspace-1', 'repo-1')).resolves.toEqual([
      { path: 'src', kind: 'directory' },
    ]);
  });
});

describe('WorkspaceApiClient Git history APIs', () => {
  it('publishes a run using repository identity instead of a client path', async () => {
    const fetcher = vi.fn<typeof fetch>().mockResolvedValue(
      new Response(
        JSON.stringify({
          id: 'run-1',
          branch: { name: 'runspace/run-1', sha: 'branch-sha' },
          commit_sha: 'commit-sha',
          pull_request: { number: 12, url: 'https://github.com/acme/app/pull/12' },
          created_at: '2026-07-29T00:00:00Z',
        }),
        { status: 202 },
      ),
    );
    const client = new WorkspaceApiClient({ fetcher });

    await client.publishRun('workspace-1', 'run-1', {
      repository_id: 'repo-1',
      branch: 'runspace/run-1',
      base: 'main',
      commit_message: 'feat: agent change',
      title: 'Agent change',
      body: '',
    });

    const request = fetcher.mock.calls[0]?.[1];
    expect(request?.method).toBe('POST');
    expect(request?.body).toContain('"repository_id":"repo-1"');
    expect(request?.body).not.toContain('repository_path');
  });

  it('loads structured changes and a path-scoped diff', async () => {
    const fetcher = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ changes: [{ path: 'src/app.ts', status: 'modified' }] }), {
          status: 200,
        }),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ path: 'src/app.ts', original: 'old', modified: 'new' }), {
          status: 200,
        }),
      );
    const client = new WorkspaceApiClient({ fetcher });

    await expect(client.listChanges('workspace-1', 'repo-1')).resolves.toHaveLength(1);
    await expect(client.readDiff('workspace-1', 'repo-1', 'src/app.ts')).resolves.toMatchObject({
      original: 'old',
      modified: 'new',
    });
    expect(fetcher.mock.calls[1]?.[0]).toContain('diff?path=src%2Fapp.ts');
  });

  it('restores durable runs and ordered outputs for a channel thread', async () => {
    const fetcher = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            runs: [
              {
                id: 'run-1',
                workspace_id: 'workspace-1',
                thread_id: 'thread-1',
                prompt: 'Build it',
                status: 'succeeded',
              },
            ],
          }),
          { status: 200 },
        ),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            outputs: [
              {
                id: 'output-1',
                run_id: 'run-1',
                kind: 'output',
                text: 'Done',
                sequence: 1,
                created_at: '2026-07-29T00:00:00Z',
              },
            ],
          }),
          { status: 200 },
        ),
      );
    const client = new WorkspaceApiClient({ fetcher });

    await expect(client.listRuns('thread-1', 'workspace-1')).resolves.toMatchObject([
      { id: 'run-1', status: 'succeeded' },
    ]);
    await expect(client.listRunOutputs('run-1')).resolves.toMatchObject([
      { id: 'output-1', sequence: 1, text: 'Done' },
    ]);
  });
});

describe('eventToTimelineItem', () => {
  it('maps message events and ignores unrelated or malformed events', () => {
    const item = eventToTimelineItem({
      id: 'event-1',
      type: 'message.created',
      workspace_id: 'atlas',
      actor_id: 'codex',
      actor_type: 'agent',
      occurred_at: '2025-01-01T10:00:00Z',
      payload: { id: 'message-1', body: 'Done' },
    });
    expect(item).toMatchObject({ author: 'Codex', provider: 'Agent', role: 'agent' });
    expect(eventToTimelineItem({ ...itemEvent(), type: 'run.started' })).toBeUndefined();
    expect(eventToTimelineItem({ ...itemEvent(), payload: null })).toBeUndefined();
  });

  it('maps ACP output and run context from NATS events', () => {
    const event = {
      ...itemEvent(),
      type: 'agent.output',
      payload: {
        RunID: 'run-1',
        Text: 'Implemented the change',
        thread_id: 'thread-1',
        channel_id: 'channel-1',
      },
    };

    expect(eventToTimelineItem(event)).toMatchObject({
      author: 'Agent',
      role: 'agent',
      body: 'Implemented the change',
    });
    expect(eventRunContext(event)).toMatchObject({
      runID: 'run-1',
      threadID: 'thread-1',
      channelID: 'channel-1',
    });
  });
});

function itemEvent() {
  return {
    id: 'event-1',
    type: 'message.created',
    workspace_id: 'atlas',
    actor_id: 'u',
    actor_type: 'user',
    occurred_at: 'bad',
    payload: {},
  };
}
