import { describe, expect, it } from 'vitest';
import { messageToTimelineItem, repositoryIDsRequiringClone } from './channel-actions';

const message = {
  id: 'message-1',
  actor_id: 'alice',
  actor_type: 'user',
  body: 'Review this',
  created_at: '2026-07-29T00:00:00Z',
};

describe('messageToTimelineItem', () => {
  it('labels only the current actor as You', () => {
    expect(messageToTimelineItem(message, 'alice').author).toBe('You');
    expect(messageToTimelineItem(message, 'bob').author).toBe('Alice');
  });

  it('preserves agent identity', () => {
    expect(
      messageToTimelineItem({ ...message, actor_id: '@reviewer', actor_type: 'agent' }, 'alice'),
    ).toMatchObject({ author: 'Reviewer', provider: 'Agent', role: 'agent' });
  });

  it('renders safe agent activity as system context', () => {
    expect(
      messageToTimelineItem(
        { ...message, actor_id: 'local-agent', actor_type: 'activity' },
        'alice',
        [
          {
            owner_id: 'nahid',
            id: 'local-agent',
            registry_id: 'codex-acp',
            name: 'Codex',
            protocol: 'acp',
            placement: 'host',
            status: 'ready',
            capabilities: [],
            updated_at: '2026-07-29T00:00:00Z',
          },
        ],
      ),
    ).toMatchObject({
      author: 'Nahid',
      provider: 'Codex',
      providerID: 'codex-acp',
      role: 'agent',
      activity: true,
      body: 'Review this',
    });
  });
});

describe('repositoryIDsRequiringClone', () => {
  const repositories = [
    { id: 'remote', fullName: 'owner/remote', defaultBranch: 'main', provider: 'github' as const },
    { id: 'mirror', fullName: 'D:\\code', defaultBranch: 'main', provider: 'mirror' as const },
    { id: 'folder', fullName: 'D:\\notes', defaultBranch: '', provider: 'folder' as const },
  ];

  it('clones remote repositories but never local host folders', () => {
    expect(repositoryIDsRequiringClone(repositories, ['remote', 'mirror', 'folder'])).toEqual([
      'remote',
    ]);
  });

  it('does not clone an unknown repository from a stale render', () => {
    expect(repositoryIDsRequiringClone(repositories, ['just-connected-local'])).toEqual([]);
  });
});
