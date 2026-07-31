import type { LocalRepositoryStatus } from '@/lib/host-agent-client';
import type { RepositoryMethod } from './channel-connection-forms';

export async function resolveConnectionTargets({
  method,
  repositoryIDs,
  repositoryURL,
  localPath,
  localPaths,
  inspectPath,
}: {
  method: RepositoryMethod;
  repositoryIDs: readonly string[];
  repositoryURL: string;
  localPath: string;
  localPaths: readonly string[];
  inspectPath: (path: string) => Promise<LocalRepositoryStatus | undefined>;
}): Promise<{ repositoryIDs: readonly string[]; repositoryURLs: readonly string[] } | undefined> {
  if (method === 'existing') {
    return repositoryIDs.length > 0 ? { repositoryIDs, repositoryURLs: [] } : undefined;
  }
  if (method === 'git') {
    const url = repositoryURL.trim();
    return url ? { repositoryIDs: [], repositoryURLs: [url] } : undefined;
  }
  if (localPath.trim()) {
    const status = await inspectPath(localPath);
    if (!status?.can_connect) return undefined;
  }
  const paths = [...new Set([...localPaths, localPath.trim()].filter(Boolean))];
  return paths.length > 0
    ? { repositoryIDs: [], repositoryURLs: paths.map((path) => `local:${path}`) }
    : undefined;
}
