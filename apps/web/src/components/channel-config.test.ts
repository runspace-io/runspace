import { describe, expect, it } from 'vitest';
import type { ApiChannel } from '@/lib/api-client';
import {
  channelAgentCommand,
  channelAgentProtocol,
  channelRepositoryIDs,
  channelSettingsDraft,
  repositoryConnectionDraft,
  repositoryConnectionsDraft,
  agentConnectionDraft,
} from './channel-config';

const channel: ApiChannel = {
  id: 'channel-1',
  workspace_id: 'workspace-1',
  name: 'build',
  repository_ids: ['repository-1'],
  config: { agent: { protocol: 'acp', command: 'codex-acp' } },
};

describe('channel config', () => {
  it('reads repository and agent connections', () => {
    expect(channelRepositoryIDs(channel)).toEqual(['repository-1']);
    expect(channelAgentProtocol(channel)).toBe('acp');
    expect(channelAgentCommand(channel)).toBe('codex-acp');
  });

  it('connects several repositories without replacing existing connections', () => {
    const draft = repositoryConnectionsDraft(channel, ['repository-2', 'repository-3']);
    expect(draft.repositoryIDs).toEqual(['repository-1', 'repository-2', 'repository-3']);
  });

  it('builds safe partial channel updates', () => {
    expect(channelSettingsDraft(channel, { name: 'review' })).toEqual({
      name: 'review',
      repositoryID: 'repository-1',
      repositoryIDs: ['repository-1'],
      repositoryURL: '',
      agentProtocol: 'acp',
      agentCommand: 'codex-acp',
    });
  });

  it('preserves unrelated connections', () => {
    expect(repositoryConnectionDraft(channel, 'repository-2', '').repositoryIDs).toEqual([
      'repository-1',
      'repository-2',
    ]);
    expect(agentConnectionDraft(channel, 'mock', '')).toMatchObject({
      repositoryIDs: ['repository-1'],
      agentProtocol: 'mock',
    });
  });
});
