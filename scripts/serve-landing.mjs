// Serves the built Hugo landing site for the e2e suite.
//
// landing.spec.ts targets the marketing site, not the app, but both used to run
// against the same baseURL — so those tests could never pass and were failing
// permanently. Giving the landing project its own server and port fixes that
// without pulling in a static-server dependency.
import { createServer } from 'node:http';
import { createReadStream, existsSync, statSync } from 'node:fs';
import { extname, join, normalize } from 'node:path';

const ROOT = normalize('landing/public');
const PORT = Number(process.env.LANDING_PORT ?? 4321);
const TYPES = {
  '.html': 'text/html; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8',
  '.svg': 'image/svg+xml',
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.woff2': 'font/woff2',
  '.xml': 'application/xml',
};

if (!existsSync(join(ROOT, 'index.html'))) {
  console.error(`No built landing site at ${ROOT}. Run: hugo --source landing`);
  process.exit(1);
}

createServer((request, response) => {
  const requested = decodeURIComponent(new URL(request.url ?? '/', 'http://localhost').pathname);
  // Contain every request inside ROOT so a traversal cannot escape it.
  const candidate = normalize(join(ROOT, requested));
  if (!candidate.startsWith(ROOT)) {
    response.writeHead(403).end('forbidden');
    return;
  }
  const file =
    existsSync(candidate) && statSync(candidate).isDirectory()
      ? join(candidate, 'index.html')
      : candidate;
  if (!existsSync(file)) {
    response.writeHead(404).end('not found');
    return;
  }
  response.writeHead(200, { 'Content-Type': TYPES[extname(file)] ?? 'application/octet-stream' });
  createReadStream(file).pipe(response);
}).listen(PORT, () => console.log(`landing site on http://localhost:${PORT}`));
