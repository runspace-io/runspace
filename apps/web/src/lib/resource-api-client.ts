import type { ApiRepositoryFile, ApiRepositorySync } from './api-types';
import { normalizeResource } from './api-normalizers';
import { RepositoryReviewClient } from './repository-review-client';
import type { ResourceSummary, WorkspaceTreeEntry } from './workspace-state';
import type { LocalAgentInstallation } from './host-agent-client';

export class ResourceApiClient extends RepositoryReviewClient {
  public listLocalAgents(): Promise<LocalAgentInstallation[]> {
    return this.request<{ agents: LocalAgentInstallation[] }>('/users/me/agents').then(
      (data) => data.agents ?? [],
    );
  }

  public assignLocalAgent(agent: LocalAgentInstallation): Promise<LocalAgentInstallation> {
    return this.request<LocalAgentInstallation>(
      `/users/me/agents/${encodeURIComponent(agent.id)}`,
      {
        method: 'PUT',
        body: JSON.stringify({
          id: agent.id,
          registry_id: agent.registry_id,
          name: agent.name,
          description: agent.description,
          protocol: agent.protocol,
          placement: agent.placement,
          status: agent.status,
          capabilities: agent.capabilities,
        }),
      },
    );
  }

  public terminalURL(workspaceID: string, resourceID: string): string {
    const terminalBase = process.env.NEXT_PUBLIC_WS_API_URL ?? this.baseURL;
    const url = new URL(
      `${terminalBase}/workspaces/${encodeURIComponent(workspaceID)}/resources/${encodeURIComponent(resourceID)}/terminal`,
      window.location.origin,
    );
    url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
    // Identity rides as ?access_token=, appended when the socket opens: a
    // browser cannot set headers on a WebSocket, and the token is short lived.
    return url.toString();
  }

  public listResources(workspaceID: string): Promise<ResourceSummary[]> {
    return this.request<{ resources: ResourceSummary[] }>(
      `/workspaces/${encodeURIComponent(workspaceID)}/resources`,
    ).then((data) => data.resources.map(normalizeResource));
  }

  public listTree(
    workspaceID: string,
    resourceID: string,
    path = '',
  ): Promise<WorkspaceTreeEntry[]> {
    const query = path ? `?path=${encodeURIComponent(path)}` : '';
    return this.request<{
      entries: Array<{ path: string; kind: string; ignored?: boolean }>;
    }>(
      `/workspaces/${encodeURIComponent(workspaceID)}/resources/${encodeURIComponent(resourceID)}/tree${query}`,
    ).then((data) =>
      data.entries
        .filter((entry) => !entry.ignored && (entry.kind === 'file' || entry.kind === 'directory'))
        .map((entry) => ({ path: entry.path, kind: entry.kind as WorkspaceTreeEntry['kind'] })),
    );
  }

  public readFile(
    workspaceID: string,
    resourceID: string,
    path: string,
  ): Promise<ApiRepositoryFile> {
    const query = new URLSearchParams({ path });
    return this.request<ApiRepositoryFile>(
      `/workspaces/${encodeURIComponent(workspaceID)}/resources/${encodeURIComponent(resourceID)}/file?${query.toString()}`,
    );
  }

  public repositorySync(workspaceID: string, resourceID: string): Promise<ApiRepositorySync> {
    return this.request<ApiRepositorySync>(
      `/workspaces/${encodeURIComponent(workspaceID)}/resources/${encodeURIComponent(resourceID)}/sync`,
    );
  }

  public connectResource(workspaceID: string, resource: ResourceSummary): Promise<ResourceSummary> {
    return this.request<ResourceSummary>(
      `/workspaces/${encodeURIComponent(workspaceID)}/resources`,
      {
        method: 'POST',
        body: JSON.stringify({
          provider: resource.provider,
          full_name: resource.fullName,
          clone_url: resource.cloneURL ?? `https://github.com/${resource.fullName}.git`,
          default_branch: resource.defaultBranch,
        }),
      },
    ).then(normalizeResource);
  }

  public prepareResource(
    workspaceID: string,
    resourceID: string,
  ): Promise<{ path: string; ref: string }> {
    return this.request<{ path: string; ref: string }>(
      `/workspaces/${encodeURIComponent(workspaceID)}/resources/${encodeURIComponent(resourceID)}/clone`,
      { method: 'POST' },
    );
  }
}
