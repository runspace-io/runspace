/**
 * Returns where to land after signing in.
 *
 * The auth middleware records the page the visitor actually wanted in
 * ?callbackUrl, which the sign-in page used to discard — so every deep link, an
 * invitation link most of all, dumped people on the workspace root instead.
 *
 * Only same-origin paths are honoured. An absolute or protocol-relative value
 * would turn the sign-in page into an open redirect.
 */
export function safeCallbackUrl(): string {
  if (typeof window === 'undefined') return '/';
  const requested = new URLSearchParams(window.location.search).get('callbackUrl');
  if (!requested) return '/';
  const path = requested.startsWith(window.location.origin)
    ? requested.slice(window.location.origin.length)
    : requested;
  return path.startsWith('/') && !path.startsWith('//') ? path : '/';
}
