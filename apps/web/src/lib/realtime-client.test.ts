import { describe, expect, it, vi } from 'vitest';
import { ReconnectingRealtimeSocket, type RealtimeSocket } from './realtime-client';
import { MAX_RETRY_DELAY_MS, retryDelay } from './retry-policy';

function fakeSocket(): RealtimeSocket {
  return {
    readyState: 0,
    onopen: null,
    onmessage: null,
    onclose: null,
    onerror: null,
    send: vi.fn(),
    close: vi.fn(),
  };
}

describe('retry policy', () => {
  it('caps exponential reconnect delay and applies bounded jitter', () => {
    expect(retryDelay(0, () => 0)).toBe(200);
    expect(retryDelay(50, () => 1)).toBe(MAX_RETRY_DELAY_MS);
  });
});

describe('ReconnectingRealtimeSocket', () => {
  it('reconnects indefinitely and preserves the last event cursor', async () => {
    const sockets: RealtimeSocket[] = [];
    const urls: string[] = [];
    const scheduled: Array<() => void> = [];
    const statuses: string[] = [];
    const client = new ReconnectingRealtimeSocket({
      url: 'ws://localhost/realtime',
      workspaceID: 'workspace-1',
      userID: 'dev-user',
      createSocket: (url) => {
        urls.push(url);
        const socket = fakeSocket();
        sockets.push(socket);
        return socket;
      },
      schedule: (callback) => {
        scheduled.push(callback);
        return scheduled.length as unknown as ReturnType<typeof setTimeout>;
      },
      cancel: vi.fn(),
      random: () => 0,
      onStatus: (status) => statuses.push(status),
    });

    client.start();
    // Opening the socket now awaits a gateway token, so the connection is made
    // on a later tick. Identity travels as ?access_token=, never ?user_id=.
    await Promise.resolve();
    expect(urls[0]).toContain('workspace_id=workspace-1');
    expect(urls[0]).not.toContain('user_id=');
    sockets[0]!.readyState = 1;
    sockets[0]!.onopen?.();
    sockets[0]!.onmessage?.({ data: JSON.stringify({ type: 'event', event: { id: 'event-9' } }) });
    sockets[0]!.onclose?.();
    scheduled[0]!();
    await Promise.resolve();
    expect(sockets).toHaveLength(2);
    expect(statuses).toContain('reconnecting');
    expect(client.lastEventID).toBe('event-9');
    expect(sockets[1]!.send).not.toHaveBeenCalled();
    client.stop();
  });
});
