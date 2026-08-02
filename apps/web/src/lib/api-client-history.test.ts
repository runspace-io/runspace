import { describe, expect, it, vi } from 'vitest';
import { eventToTimelineItem, WorkspaceApiClient } from './api-client';
import { eventRunContext } from './api-normalizers';

// The transport exchanges a session for a gateway token; tests supply one
// directly so the mocked fetcher only ever sees the call under test.
const tokenSource = () => Promise.resolve('test-gateway-token');

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
    const client = new WorkspaceApiClient({ fetcher, tokenSource });

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
    const client = new WorkspaceApiClient({ fetcher, tokenSource });

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
    const client = new WorkspaceApiClient({ fetcher, tokenSource });

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
