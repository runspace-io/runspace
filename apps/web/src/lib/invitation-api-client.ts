import type { ApiInvitation, ApiInvitationPreview, ApiMember } from './api-types';
import { AgentTaskApiClient } from './agent-task-api-client';

export class InvitationApiClient extends AgentTaskApiClient {
  /**
   * The token comes back exactly once — only its hash is stored — so the caller
   * must show or copy it immediately.
   */
  public createInvitation(
    workspaceID: string,
    role: ApiInvitation['role'] = 'member',
  ): Promise<{ invitation: ApiInvitation; token: string }> {
    return this.request(`/workspaces/${encodeURIComponent(workspaceID)}/invitations`, {
      method: 'POST',
      body: JSON.stringify({ role }),
    });
  }

  public listInvitations(workspaceID: string): Promise<ApiInvitation[]> {
    return this.request<{ invitations: ApiInvitation[] | null }>(
      `/workspaces/${encodeURIComponent(workspaceID)}/invitations`,
    ).then((data) => data.invitations ?? []);
  }

  public revokeInvitation(workspaceID: string, invitationID: string): Promise<void> {
    return this.request<void>(
      `/workspaces/${encodeURIComponent(workspaceID)}/invitations/${encodeURIComponent(invitationID)}`,
      { method: 'DELETE' },
    );
  }

  // Tokens travel in the body, never the path, so they stay out of access logs.
  public previewInvitation(token: string): Promise<ApiInvitationPreview> {
    return this.request<ApiInvitationPreview>('/invitations/preview', {
      method: 'POST',
      body: JSON.stringify({ token }),
    });
  }

  public acceptInvitation(token: string): Promise<ApiMember> {
    return this.request<ApiMember>('/invitations/accept', {
      method: 'POST',
      body: JSON.stringify({ token }),
    });
  }
}
