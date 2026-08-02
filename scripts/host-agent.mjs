// Runs the host agent with the same .env the stack uses.
//
// The gateway authenticates every caller, and the host agent signs its pushes
// with GATEWAY_AUTH_SECRET. Compose loads .env for the containers; a bare
// `go run` does not, so without this the agent's presence and task pushes are
// rejected and it silently looks offline in the web app.
import { spawn } from 'node:child_process';
import { readFileSync } from 'node:fs';

function envFromFile(path = '.env') {
  const values = {};
  try {
    for (const line of readFileSync(path, 'utf8').split(/\r?\n/u)) {
      const trimmed = line.trim();
      if (!trimmed || trimmed.startsWith('#')) continue;
      const separator = trimmed.indexOf('=');
      if (separator > 0) values[trimmed.slice(0, separator).trim()] = trimmed.slice(separator + 1);
    }
  } catch {
    console.warn('No .env found; the host agent may not be able to authenticate.');
  }
  return values;
}

// Real environment variables win, so an override on the command line still works.
const child = spawn('go', ['run', './cmd/host-agent'], {
  stdio: 'inherit',
  env: { ...envFromFile(), ...process.env },
  shell: process.platform === 'win32',
});
child.on('exit', (code) => process.exit(code ?? 1));
