import type { WorkspaceTreeEntry } from '@/lib/workspace-state';

export function expandRepositoryDirectory(
  entries: readonly WorkspaceTreeEntry[],
  directory: string,
  children: readonly WorkspaceTreeEntry[],
): WorkspaceTreeEntry[] {
  const directoryIndex = entries.findIndex((entry) => entry.path === directory);
  if (directoryIndex < 0) return [...entries];

  const withoutDescendants = entries.filter((entry) => !isDescendant(entry.path, directory));
  const insertionIndex = withoutDescendants.findIndex((entry) => entry.path === directory) + 1;
  return [
    ...withoutDescendants.slice(0, insertionIndex),
    ...children,
    ...withoutDescendants.slice(insertionIndex),
  ];
}

export function collapseRepositoryDirectory(
  entries: readonly WorkspaceTreeEntry[],
  directory: string,
): WorkspaceTreeEntry[] {
  return entries.filter((entry) => !isDescendant(entry.path, directory));
}

export function replaceRepositoryRoot(
  entries: readonly WorkspaceTreeEntry[],
  rootEntries: readonly WorkspaceTreeEntry[],
): WorkspaceTreeEntry[] {
  return rootEntries.flatMap((root) => [
    root,
    ...entries.filter((entry) => isDescendant(entry.path, root.path)),
  ]);
}

export function collapsedDirectoryPaths(expanded: readonly string[], directory: string): string[] {
  return expanded.filter((path) => path !== directory && !isDescendant(path, directory));
}

function isDescendant(path: string, directory: string): boolean {
  return path.startsWith(`${directory}/`);
}
