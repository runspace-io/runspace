import { describe, expect, it } from 'vitest';
import { validateChannelDraft, type ChannelDraft } from './channel-model';

const valid: ChannelDraft = {
  name: 'engineering',
  parentID: '',
  repositoryID: '',
  repositoryIDs: [],
  repositoryURL: '',
  agentProtocol: 'mock',
  agentCommand: '',
};

describe('validateChannelDraft', () => {
  it('accepts a functional development-agent channel', () => {
    expect(validateChannelDraft(valid)).toBeUndefined();
  });

  it('rejects missing names and permits appending a repository', () => {
    expect(validateChannelDraft({ ...valid, name: '' })).toBe('Channel name is required.');
    expect(
      validateChannelDraft({
        ...valid,
        repositoryID: 'repository-1',
        repositoryIDs: ['repository-1'],
        repositoryURL: 'https://github.com/runspace/runspace',
      }),
    ).toBeUndefined();
  });
});
