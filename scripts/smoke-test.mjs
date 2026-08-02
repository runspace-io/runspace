// Runs the smoke test with whichever shell this platform has, so callers do not
// have to remember which of smoke-test.sh / smoke-test.ps1 applies to them.
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const here = dirname(fileURLToPath(import.meta.url));
const windows = process.platform === 'win32';
const [command, args] = windows
  ? [
      'powershell',
      ['-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', join(here, 'smoke-test.ps1')],
    ]
  : ['bash', [join(here, 'smoke-test.sh')]];

const result = spawnSync(command, args, { stdio: 'inherit' });
if (result.error) {
  console.error(`Could not run the smoke test with ${command}: ${result.error.message}`);
  process.exit(1);
}
process.exit(result.status ?? 1);
