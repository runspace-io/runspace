import { describe, expect, it, vi } from 'vitest';
import { ApiError, WorkspaceApiClient } from './api-client';

// The transport exchanges a session for a gateway token; tests supply one
// directly so the mocked fetcher only ever sees the call under test.
const tokenSource = () => Promise.resolve('test-gateway-token');

describe('WorkspaceApiClient', () => {
  it('retries transient responses and sends a signed bearer token', async () => {
    const fetcher = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(new Response('temporarily unavailable', { status: 503 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ workspaces: [] }), { status: 200 }));
    const client = new WorkspaceApiClient({
      fetcher,
      tokenSource,
      sleep: vi.fn().mockResolvedValue(undefined),
    });

    await expect(client.listWorkspaces()).resolves.toEqual([]);
    expect(fetcher).toHaveBeenCalledTimes(2);
    // Identity is a signed bearer token now; the old x-user-id header was
    // client-controlled and is deliberately gone.
    expect(fetcher.mock.calls[0]?.[1]?.headers).toMatchObject({
      authorization: 'Bearer test-gateway-token',
    });
    expect(fetcher.mock.calls[0]?.[1]?.headers).not.toHaveProperty('x-user-id');
  });

  it('does not retry permanent client errors', async () => {
    const fetcher = vi
      .fn<typeof fetch>()
      .mockResolvedValue(new Response('forbidden', { status: 403 }));
    const client = new WorkspaceApiClient({ fetcher, tokenSource, sleep: vi.fn() });

    await expect(client.listWorkspaces()).rejects.toEqual(new ApiError(403, 'forbidden'));
    expect(fetcher).toHaveBeenCalledTimes(1);
  });

  it('surfaces structured API errors as readable messages', async () => {
    const fetcher = vi
      .fn<typeof fetch>()
      .mockResolvedValue(
        new Response(JSON.stringify({ error: 'repository is not connected' }), { status: 400 }),
      );
    const client = new WorkspaceApiClient({ fetcher, tokenSource });

    await expect(client.listWorkspaces()).rejects.toEqual(
      new ApiError(400, 'repository is not connected'),
    );
  });

  it('normalizes an empty message response from the API', async () => {
    const fetcher = vi
      .fn<typeof fetch>()
      .mockResolvedValue(new Response(JSON.stringify({ messages: null }), { status: 200 }));
    const client = new WorkspaceApiClient({ fetcher, tokenSource });

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
    const client = new WorkspaceApiClient({ fetcher, tokenSource });

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
    const client = new WorkspaceApiClient({ fetcher, tokenSource });

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
    const client = new WorkspaceApiClient({ fetcher, tokenSource });
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
    const client = new WorkspaceApiClient({ fetcher, tokenSource });

    await expect(client.listTree('workspace-1', 'repo-1')).resolves.toEqual([
      { path: 'src', kind: 'directory' },
    ]);
  });
});
