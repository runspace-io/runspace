import { NextResponse } from 'next/server';
import { getToken } from 'next-auth/jwt';
import type { NextRequest } from 'next/server';
import { gatewayAuthSecret, issueGatewayToken } from '@/lib/gateway-token';

/**
 * Exchanges a NextAuth session for a short-lived gateway token.
 *
 * This is the only place a session becomes a gateway identity. getToken()
 * verifies the session cookie's signature, so the subject here is proven rather
 * than claimed — which is the whole point of the exchange.
 */
export async function POST(request: NextRequest) {
  const secret = gatewayAuthSecret();
  if (!secret) {
    return NextResponse.json(
      { error: 'gateway authentication is not configured' },
      { status: 500 },
    );
  }
  const session = await getToken({ req: request, secret: process.env.NEXTAUTH_SECRET ?? secret });
  const subject = typeof session?.sub === 'string' ? session.sub : '';
  if (!subject) {
    return NextResponse.json({ error: 'authentication required' }, { status: 401 });
  }
  const { token, expiresAt } = issueGatewayToken(subject, secret);
  return NextResponse.json(
    { token, expires_at: expiresAt },
    // Never cached: it is a credential, and it expires quickly.
    { headers: { 'Cache-Control': 'no-store' } },
  );
}
