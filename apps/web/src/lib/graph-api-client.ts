import type {
  ApiGraphContext,
  ApiGraphKind,
  ApiGraphNode,
  ApiResourceConnection,
  ApiResourcePlugin,
  ApiUIDocument,
} from './api-types';
import { AgentTaskApiClient } from './agent-task-api-client';

export class GraphApiClient extends AgentTaskApiClient {
  public requestUIAction(
    workspaceID: string,
    input: {
      operation: string;
      resource: string;
      reason?: string;
      thread_id?: string;
      channel_id?: string;
    },
  ): Promise<ApiGraphNode> {
    return this.request(`/workspaces/${encodeURIComponent(workspaceID)}/ui/actions`, {
      method: 'POST',
      body: JSON.stringify(input),
    });
  }

  public createUIArtifact(
    workspaceID: string,
    document: ApiUIDocument,
    context: { thread_id?: string; channel_id?: string } = {},
  ): Promise<ApiGraphNode> {
    return this.request(`/workspaces/${encodeURIComponent(workspaceID)}/ui/artifacts`, {
      method: 'POST',
      body: JSON.stringify({ document, ...context }),
    });
  }

  public listResourcePlugins(): Promise<ApiResourcePlugin[]> {
    return this.request<{ plugins: ApiResourcePlugin[] }>('/resource-plugins').then(
      (result) => result.plugins ?? [],
    );
  }

  public connectNativeResource(
    workspaceID: string,
    input: {
      plugin_id: ApiResourcePlugin['id'];
      title: string;
      placement: 'runspace';
      auth_method: string;
      access_mode: 'read' | 'manage' | 'full';
      credential: string;
    },
  ): Promise<ApiResourceConnection> {
    return this.request(`/workspaces/${encodeURIComponent(workspaceID)}/resource-connections`, {
      method: 'POST',
      body: JSON.stringify(input),
    });
  }

  public listGraphNodes(
    workspaceID: string,
    query: {
      kind?: ApiGraphKind;
      type?: string;
      text?: string;
      threadID?: string;
      limit?: number;
    } = {},
  ): Promise<ApiGraphNode[]> {
    const params = new URLSearchParams();
    if (query.kind) params.set('kind', query.kind);
    if (query.type) params.set('type', query.type);
    if (query.text) params.set('q', query.text);
    if (query.threadID) params.set('thread_id', query.threadID);
    if (query.limit) params.set('limit', String(query.limit));
    const suffix = params.size ? `?${params.toString()}` : '';
    return this.request<{ nodes: ApiGraphNode[] }>(
      `/workspaces/${encodeURIComponent(workspaceID)}/graph/nodes${suffix}`,
    ).then((data) => data.nodes ?? []);
  }

  public getGraphContext(workspaceID: string, nodeID: string): Promise<ApiGraphContext> {
    return this.request<ApiGraphContext>(
      `/workspaces/${encodeURIComponent(workspaceID)}/graph/nodes/${encodeURIComponent(nodeID)}`,
    );
  }

  public queryGraphResource(
    workspaceID: string,
    nodeID: string,
    input: { capability: string; query: string; limit?: number },
  ): Promise<{
    resource_id: string;
    capability: string;
    matches: Array<{
      title: string;
      summary?: string;
      reference?: string;
      metadata?: Record<string, unknown>;
    }>;
    truncated: boolean;
  }> {
    return this.request(
      `/workspaces/${encodeURIComponent(workspaceID)}/graph/nodes/${encodeURIComponent(nodeID)}/query`,
      { method: 'POST', body: JSON.stringify(input) },
    );
  }

  public getGraphResourceAvailability(
    workspaceID: string,
    nodeID: string,
  ): Promise<{
    resource_id: string;
    status: 'available' | 'unavailable';
    reason?: string;
    checked_at: string;
    expires_at: string;
  }> {
    return this.request(
      `/workspaces/${encodeURIComponent(workspaceID)}/graph/nodes/${encodeURIComponent(nodeID)}/availability`,
    );
  }
}
