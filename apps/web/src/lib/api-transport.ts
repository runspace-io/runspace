import { retryDelay } from './retry-policy';

type FetchLike = typeof fetch;
type RequestOptions = NonNullable<Parameters<FetchLike>[1]>;

export type ApiClientOptions = {
  baseURL?: string;
  userID?: string;
  fetcher?: FetchLike;
  sleep?: (delay: number) => Promise<void>;
  random?: () => number;
  /** Overrides how a gateway token is obtained. Tests supply their own. */
  tokenSource?: () => Promise<string>;
};

const DEFAULT_BASE_URL = 'http://localhost:8080/api/v1';
const MAX_ATTEMPTS = 8;

export class ApiError extends Error {
  public readonly status: number;

  public constructor(status: number, message: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
  }
}

export class RetryingApiTransport {
  protected readonly baseURL: string;
  protected readonly userID: string;
  private readonly fetcher: FetchLike;
  private readonly sleep: (delay: number) => Promise<void>;
  private readonly random: () => number;
  private readonly tokenSource: () => Promise<string>;
  private cachedToken: { value: string; expiresAt: number } | undefined;
  private pendingToken: Promise<string> | undefined;

  public constructor(options: ApiClientOptions = {}) {
    this.tokenSource = options.tokenSource ?? fetchGatewayToken;
    this.baseURL = (options.baseURL ?? process.env.NEXT_PUBLIC_API_URL ?? DEFAULT_BASE_URL).replace(
      /\/$/,
      '',
    );
    this.userID = options.userID ?? process.env.NEXT_PUBLIC_USER_ID ?? 'admin';
    this.fetcher = options.fetcher ?? globalThis.fetch.bind(globalThis);
    this.sleep = options.sleep ?? ((delay) => new Promise((resolve) => setTimeout(resolve, delay)));
    this.random = options.random ?? Math.random;
  }

  public get actorID(): string {
    return this.userID;
  }

  protected async request<T>(path: string, init: RequestOptions = {}): Promise<T> {
    let lastError: unknown;
    for (let attempt = 0; attempt < MAX_ATTEMPTS; attempt += 1) {
      try {
        return await this.attempt<T>(path, init);
      } catch (error) {
        lastError = error;
        if (!isRetryable(error)) throw error;
      }
      await this.sleep(retryDelay(attempt, this.random));
    }
    throw lastError instanceof Error ? lastError : new Error('Request failed after retries');
  }

  private async attempt<T>(path: string, init: RequestOptions): Promise<T> {
    let response = await this.send(path, init, await this.token());
    if (response.status === 401) {
      // The token is short lived, so the usual 401 is simply an expired one.
      // Mint a fresh token and retry once before surfacing an auth failure.
      this.pendingToken = undefined;
      this.cachedToken = undefined;
      response = await this.send(path, init, await this.token());
    }
    if (response.ok) {
      if (response.status === 204) return undefined as T;
      return (await response.json()) as T;
    }
    const body = await response.text();
    throw new ApiError(response.status, apiErrorMessage(body, response.status));
  }

  private send(path: string, init: RequestOptions, token: string) {
    const authorization = token ? { authorization: `Bearer ${token}` } : {};
    return this.fetcher(`${this.baseURL}${path}`, {
      ...init,
      headers: { 'content-type': 'application/json', ...authorization, ...init.headers },
    });
  }

  /** The realtime socket needs the same token; it cannot set headers. */
  public gatewayToken(): Promise<string> {
    return this.token();
  }

  /** Cached so a burst of calls shares one exchange rather than racing. */
  protected async token(): Promise<string> {
    if (this.cachedToken && this.cachedToken.expiresAt > Date.now()) {
      return this.cachedToken.value;
    }
    this.pendingToken ??= this.tokenSource()
      .then((value) => {
        // Refresh a little early so a request never travels with a token that
        // expires in flight.
        this.cachedToken = { value, expiresAt: Date.now() + 5 * 60 * 1000 };
        return value;
      })
      .finally(() => {
        this.pendingToken = undefined;
      });
    return this.pendingToken;
  }
}

async function fetchGatewayToken(): Promise<string> {
  const response = await fetch('/api/auth/gateway-token', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
  });
  if (!response.ok) return '';
  const payload = (await response.json()) as { token?: string };
  return payload.token ?? '';
}

function apiErrorMessage(body: string, status: number): string {
  if (!body) return `Request failed with ${status}`;
  try {
    const parsed = JSON.parse(body) as { error?: unknown };
    if (typeof parsed.error === 'string' && parsed.error.trim()) return parsed.error;
  } catch {
    // Plain-text responses remain useful as-is.
  }
  return body;
}

function isRetryable(error: unknown): boolean {
  return !(error instanceof ApiError) || error.status >= 500 || error.status === 429;
}
