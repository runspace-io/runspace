import { readdir, readFile } from 'node:fs/promises';
import { join, relative } from 'node:path';

const limits = [
  { root: 'apps/web/src', extensions: new Set(['.ts', '.tsx']), maxLines: 300 },
  { root: 'internal', extensions: new Set(['.go']), maxLines: 300 },
  { root: 'cmd', extensions: new Set(['.go']), maxLines: 300 },
];

async function filesUnder(root) {
  const entries = await readdir(root, { withFileTypes: true });
  const files = [];
  for (const entry of entries) {
    const path = join(root, entry.name);
    if (entry.isDirectory()) files.push(...(await filesUnder(path)));
    else files.push(path);
  }
  return files;
}

const violations = [];
for (const limit of limits) {
  for (const file of await filesUnder(limit.root)) {
    const extension = file.slice(file.lastIndexOf('.'));
    if (!limit.extensions.has(extension)) continue;
    const lines = (await readFile(file, 'utf8')).split(/\r?\n/).length - 1;
    if (lines > limit.maxLines)
      violations.push(`${relative('.', file)} has ${lines} lines (max ${limit.maxLines})`);
  }
}

if (violations.length > 0) {
  console.error('Focused-file size checks failed:');
  for (const violation of violations) console.error(`- ${violation}`);
  process.exit(1);
}

console.log('Focused-file size checks passed.');
