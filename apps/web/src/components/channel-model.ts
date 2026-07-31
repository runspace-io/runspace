export type ChannelDraft = {
  name: string;
  parentID: string;
  repositoryID: string;
  repositoryIDs: string[];
  repositoryURL: string;
  agentProtocol: 'none' | 'mock' | 'acp';
  agentCommand: string;
};

export type ChannelSettingsDraft = Omit<ChannelDraft, 'parentID'>;

export function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === 'object';
}

export function isRunning(status: string | undefined): boolean {
  return status === 'running' || status === 'queued' || status === 'stopping';
}

export function validateChannelDraft(draft: ChannelDraft): string | undefined {
  const name = draft.name.trim();
  if (!name) return 'Channel name is required.';
  if (name.length > 64) return 'Channel name must be 64 characters or fewer.';
  return undefined;
}
