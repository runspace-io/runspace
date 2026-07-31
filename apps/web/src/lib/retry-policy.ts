export const MAX_RETRY_DELAY_MS = 30_000;

export function retryDelay(attempt: number, errorOrRandom?: unknown): number {
  const safeAttempt = Math.max(0, Math.floor(attempt));
  const exponential = Math.min(MAX_RETRY_DELAY_MS, 250 * 2 ** safeAttempt);
  const random =
    typeof errorOrRandom === 'function' ? (errorOrRandom as () => number) : Math.random;
  const jitter = 0.8 + random() * 0.4;
  return Math.min(MAX_RETRY_DELAY_MS, Math.round(exponential * jitter));
}
