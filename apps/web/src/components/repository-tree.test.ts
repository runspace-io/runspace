import { describe, expect, it } from 'vitest';
import {
  collapsedDirectoryPaths,
  collapseRepositoryDirectory,
  expandRepositoryDirectory,
  replaceRepositoryRoot,
} from './repository-tree';

const root = [
  { path: 'src', kind: 'directory' as const },
  { path: 'README.md', kind: 'file' as const },
];

describe('repository tree', () => {
  it('inserts children after their directory without losing root entries', () => {
    expect(
      expandRepositoryDirectory(root, 'src', [
        { path: 'src/components', kind: 'directory' },
        { path: 'src/index.ts', kind: 'file' },
      ]),
    ).toEqual([
      root[0],
      { path: 'src/components', kind: 'directory' },
      { path: 'src/index.ts', kind: 'file' },
      root[1],
    ]);
  });

  it('collapses all descendants and their expanded state', () => {
    const expanded = expandRepositoryDirectory(root, 'src', [
      { path: 'src/components', kind: 'directory' },
      { path: 'src/components/button.tsx', kind: 'file' },
    ]);
    expect(collapseRepositoryDirectory(expanded, 'src')).toEqual(root);
    expect(collapsedDirectoryPaths(['src', 'src/components', 'docs'], 'src')).toEqual(['docs']);
  });

  it('refreshes root entries without collapsing loaded directories', () => {
    expect(
      replaceRepositoryRoot(
        [
          { path: 'src', kind: 'directory' },
          { path: 'src/index.ts', kind: 'file' },
          { path: 'README.md', kind: 'file' },
        ],
        [...root, { path: 'package.json', kind: 'file' }],
      ),
    ).toEqual([
      root[0],
      { path: 'src/index.ts', kind: 'file' },
      root[1],
      { path: 'package.json', kind: 'file' },
    ]);
  });
});
