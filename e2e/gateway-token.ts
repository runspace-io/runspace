import { createHmac } from 'node:crypto';
import { readFileSync } from 'node:fs';

/**
 * Reads the secret the running stack actually uses.
 *
 * Compose loads .env; Playwright does not, so without this the suite signs with
 * a different key than the gateway verifies with and every call is a 401.
 */
function secretFromEnvFile(name: string): string | undefined {
  try {
    for (const line of readFileSync('.env', 'utf8').split(/\r?\n/u)) {
      const separator = line.indexOf('=');
      if (separator > 0 && line.slice(0, separator).trim() === name) {
        return line.slice(separator + 1).trim();
      }
    }
  } catch {
    // No .env file: fall through to the compose defaults below.
  }
  return undefined;
}

const SECRET =
  process.env.GATEWAY_AUTH_SECRET ??
  secretFromEnvFile('GATEWAY_AUTH_SECRET') ??
  process.env.NEXTAUTH_SECRET ??
  secretFromEnvFile('NEXTAUTH_SECRET') ??
  'runspace-development-secret-change-me';

function base64url(value: string): string {
  return Buffer.from(value).toString('base64url');
}

/**
 * Signs a gateway token for a test's direct API calls.
 *
 * The gateway no longer trusts an X-User-ID header, so seeding fixtures over
 * HTTP means proving identity the same way the web app does.
 */
export function gatewayToken(subject: string): string {
  const now = Math.floor(Date.now() / 1000);
  const header = base64url(JSON.stringify({ alg: 'HS256', typ: 'JWT' }));
  const payload = base64url(
    JSON.stringify({ sub: subject, iss: 'runspace-e2e', iat: now, exp: now + 600 }),
  );
  const body = `${header}.${payload}`;
  return `${body}.${createHmac('sha256', SECRET).update(body).digest('base64url')}`;
}

export function asUser(subject: string): Record<string, string> {
  return { Authorization: `Bearer ${gatewayToken(subject)}` };
}
