// Creates .env on first run with real random secrets.
//
// Copying .env.example verbatim left the stack signing gateway tokens with the
// literal string "replace-with-a-long-random-value", so anyone who had read the
// repository could mint a token for any identity. Generating them here means a
// fresh clone is safe by default instead of safe only if you remember.
import { randomBytes } from 'node:crypto';
import { copyFileSync, existsSync, readFileSync, writeFileSync } from 'node:fs';

const TARGET = '.env';
const EXAMPLE = '.env.example';
// Anything whose example value is a placeholder must not survive into .env.
const GENERATED = ['NEXTAUTH_SECRET', 'GATEWAY_AUTH_SECRET', 'CHANNEL_SECRET_KEY'];

if (existsSync(TARGET)) process.exit(0);
if (!existsSync(EXAMPLE)) {
  console.error(`Cannot create ${TARGET}: ${EXAMPLE} is missing.`);
  process.exit(1);
}

copyFileSync(EXAMPLE, TARGET);
const secret = () => randomBytes(32).toString('base64url');
const contents = readFileSync(TARGET, 'utf8')
  .split(/\r?\n/u)
  .map((line) => {
    const separator = line.indexOf('=');
    if (separator <= 0) return line;
    const key = line.slice(0, separator).trim();
    return GENERATED.includes(key) ? `${key}=${secret()}` : line;
  })
  .join('\n');
writeFileSync(TARGET, contents);
console.log(`Created ${TARGET} with freshly generated secrets.`);
