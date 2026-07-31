import { describe, expect, it } from 'vitest';
import { resolveConnectionTargets } from './repository-connection-targets';

describe('repository connection targets', () => {
  it('builds a batch of unique local mirrors', async () => {
    const targets = await resolveConnectionTargets({
      method: 'local',
      repositoryIDs: [],
      repositoryURL: '',
      localPath: '/work/three',
      localPaths: ['/work/one', '/work/two', '/work/one'],
      inspectPath: async (path) => ({
        path,
        git: false,
        has_remote: false,
        can_connect: true,
      }),
    });
    expect(targets?.repositoryURLs).toEqual([
      'local:/work/one',
      'local:/work/two',
      'local:/work/three',
    ]);
  });
});
