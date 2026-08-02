import { retryDelay } from './retry-policy';

export type RealtimeStatus = 'idle' | 'connecting' | 'connected' | 'reconnecting' | 'closed';

export type RealtimeEvent = {
  id: string;
  type: string;
  workspace_id: string;
  repository_id?: string;
  channel_id?: string;
  thread_id?: string;
  actor_id: string;
  actor_type: string;
  occurred_at: string;
  payload: unknown;
};

export type RealtimeFrame = {
  type: string;
  event?: RealtimeEvent;
  [key: string]: unknown;
};

export type RealtimeSocket = {
  readyState: number;
  onopen: ((event?: unknown) => void) | null;
  onmessage: ((event: { data: string }) => void) | null;
  onclose: ((event?: unknown) => void) | null;
  onerror: ((event?: unknown) => void) | null;
  send(data: string): void;
  close(): void;
};

export type RealtimeSocketOptions = {
  url: string;
  workspaceID: string;
  userID?: string;
  createSocket?: (url: string) => RealtimeSocket;
  /** Supplies the short-lived gateway token; omitted in tests. */
  tokenSource?: () => Promise<string>;
  onStatus?: (status: RealtimeStatus) => void;
  onFrame?: (frame: RealtimeFrame) => void;
  schedule?: (callback: () => void, delay: number) => ReturnType<typeof setTimeout>;
  cancel?: (handle: ReturnType<typeof setTimeout>) => void;
  random?: () => number;
};

const OPEN = 1;

export class ReconnectingRealtimeSocket {
  private readonly options: Required<RealtimeSocketOptions>;
  private socket: RealtimeSocket | undefined;
  private retryAttempt = 0;
  private reconnectHandle: ReturnType<typeof setTimeout> | undefined;
  private stopped = true;
  private cursor: string | undefined;

  public constructor(options: RealtimeSocketOptions) {
    this.options = {
      ...options,
      userID: options.userID ?? 'dev-user',
      createSocket:
        options.createSocket ?? ((url) => new WebSocket(url) as unknown as RealtimeSocket),
      onStatus: options.onStatus ?? (() => undefined),
      onFrame: options.onFrame ?? (() => undefined),
      schedule: options.schedule ?? ((callback, delay) => setTimeout(callback, delay)),
      cancel: options.cancel ?? ((handle) => clearTimeout(handle)),
      random: options.random ?? Math.random,
      tokenSource: options.tokenSource ?? (() => Promise.resolve('')),
    };
  }

  public start(): void {
    if (!this.stopped) return;
    this.stopped = false;
    this.connect();
  }

  public stop(): void {
    this.stopped = true;
    if (this.reconnectHandle !== undefined) this.options.cancel?.(this.reconnectHandle);
    this.reconnectHandle = undefined;
    this.socket?.close();
    this.socket = undefined;
    this.options.onStatus?.('closed');
  }

  public send(frame: RealtimeFrame): boolean {
    if (this.socket?.readyState !== OPEN) return false;
    this.socket.send(JSON.stringify(frame));
    return true;
  }

  public get lastEventID(): string | undefined {
    return this.cursor;
  }

  private connect(): void {
    if (this.stopped) return;
    this.options.onStatus?.(this.retryAttempt === 0 ? 'connecting' : 'reconnecting');
    // A browser cannot set headers on a WebSocket, so the gateway token has to
    // ride in the query string. Fetching it makes connect asynchronous.
    void this.openSocket();
  }

  private async openSocket(): Promise<void> {
    const query = new URLSearchParams({ workspace_id: this.options.workspaceID });
    if (this.cursor) query.set('last_event_id', this.cursor);
    const token = await this.options.tokenSource();
    if (this.stopped) return;
    if (token) query.set('access_token', token);
    const socket = this.options.createSocket(`${this.options.url}?${query.toString()}`);
    this.socket = socket;
    socket.onopen = () => {
      this.retryAttempt = 0;
      this.options.onStatus?.('connected');
      this.send({
        type: 'subscribe',
        workspace_id: this.options.workspaceID,
        last_event_id: this.cursor,
      });
    };
    socket.onmessage = (message) => this.receive(message.data);
    socket.onerror = () => socket.close();
    socket.onclose = () => this.scheduleReconnect();
  }

  private receive(data: string): void {
    try {
      const frame = JSON.parse(data) as RealtimeFrame;
      if (frame.event?.id) this.cursor = frame.event.id;
      this.options.onFrame?.(frame);
    } catch {
      // Invalid frames are ignored. Durable state is refreshed after reconnect.
    }
  }

  private scheduleReconnect(): void {
    if (this.stopped || this.reconnectHandle !== undefined) return;
    const delay = retryDelay(this.retryAttempt, this.options.random);
    this.retryAttempt += 1;
    this.reconnectHandle = this.options.schedule?.(() => {
      this.reconnectHandle = undefined;
      this.connect();
    }, delay);
  }
}
