import { createHmac } from 'node:crypto';

/** Short lived: the token is only a bearer of identity, so limit the blast
 * radius if one leaks through a log or a WebSocket query string. */
export const GATEWAY_TOKEN_TTL_SECONDS = 10 * 60;

function base64url(input: Buffer | string): string {
  return Buffer.from(input).toString('base64url');
}

/**
 * Mints the token the browser presents to the gateway.
 *
 * The gateway used to trust an X-User-ID header, so any client could act as
 * anyone. Only the web app can verify a NextAuth session, so it signs the
 * verified subject with a secret it shares with the gateway.
 */
export function issueGatewayToken(
  subject: string,
  secret: string,
  nowSeconds = Math.floor(Date.now() / 1000),
): { token: string; expiresAt: number } {
  if (!subject.trim()) throw new Error('a gateway token needs a subject');
  if (!secret.trim()) throw new Error('GATEWAY_AUTH_SECRET is not configured');
  const expiresAt = nowSeconds + GATEWAY_TOKEN_TTL_SECONDS;
  // The header is fixed so a token can never negotiate its own algorithm.
  const header = base64url(JSON.stringify({ alg: 'HS256', typ: 'JWT' }));
  const payload = base64url(
    JSON.stringify({ sub: subject, iss: 'runspace-web', iat: nowSeconds, exp: expiresAt }),
  );
  const body = `${header}.${payload}`;
  const signature = createHmac('sha256', secret).update(body).digest('base64url');
  return { token: `${body}.${signature}`, expiresAt };
}

export function gatewayAuthSecret(): string {
  return process.env.GATEWAY_AUTH_SECRET ?? process.env.NEXTAUTH_SECRET ?? '';
}
