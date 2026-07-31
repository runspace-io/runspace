import { execFileSync } from 'node:child_process';
import { mkdirSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';

export function createLocalRepositoryFixture(
  options: { name?: string; marker?: string } = {},
): string {
  const suffix = options.name ? `-${options.name}` : '';
  const name = `local-repository-${process.pid}-${Date.now()}${suffix}`;
  const path = resolve('test-results', name);
  mkdirSync(resolve(path, 'src'), { recursive: true });
  writeFileSync(
    resolve(path, 'README.md'),
    `# Runspace E2E fixture\n${options.marker ?? 'DEFAULT_REPOSITORY'}\n`,
  );
  writeFileSync(resolve(path, 'src', 'index.ts'), 'export const ready = true;\n');
  runGit(path, 'init', '--initial-branch=main');
  runGit(path, 'config', 'user.email', 'e2e@runspace.local');
  runGit(path, 'config', 'user.name', 'Runspace E2E');
  runGit(path, 'add', '.');
  runGit(path, 'commit', '-m', 'test: seed local repository');
  return `file:///src/test-results/${name}`;
}

export function createHostFolderFixture(options: { name: string; marker: string }): string {
  const name = `host-folder-${process.pid}-${Date.now()}-${options.name}`;
  const path = resolve('test-results', name);
  mkdirSync(resolve(path, 'nested'), { recursive: true });
  writeFileSync(resolve(path, 'README.md'), `# Host folder\n${options.marker}\n`);
  writeFileSync(resolve(path, 'nested', `${options.name}.txt`), `${options.marker}_NESTED\n`);
  return path;
}

export function restartGateway(): void {
  execFileSync(
    'docker',
    ['compose', '-f', 'docker-compose.yml', '-f', 'docker-compose.dev.yml', 'restart', 'gateway'],
    { cwd: resolve('.'), stdio: 'ignore' },
  );
}

function runGit(path: string, ...args: string[]): void {
  execFileSync('git', args, { cwd: path, stdio: 'ignore' });
}
