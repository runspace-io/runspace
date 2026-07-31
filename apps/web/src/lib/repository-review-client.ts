import type { ApiRepositoryChange, ApiRepositoryDiff } from './api-types';
import { RetryingApiTransport } from './api-transport';

export class RepositoryReviewClient extends RetryingApiTransport {
  public listChanges(workspaceID: string, repositoryID: string): Promise<ApiRepositoryChange[]> {
    return this.request<{ changes: ApiRepositoryChange[] }>(
      `/workspaces/${encodeURIComponent(workspaceID)}/resources/${encodeURIComponent(repositoryID)}/changes`,
    ).then((data) => data.changes);
  }

  public readDiff(
    workspaceID: string,
    repositoryID: string,
    path: string,
  ): Promise<ApiRepositoryDiff> {
    const query = new URLSearchParams({ path });
    return this.request<ApiRepositoryDiff>(
      `/workspaces/${encodeURIComponent(workspaceID)}/resources/${encodeURIComponent(repositoryID)}/diff?${query.toString()}`,
    );
  }
}
