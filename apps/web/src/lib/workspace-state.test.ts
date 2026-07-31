import { describe, expect, it } from 'vitest';
import {
  parseGithubRepository,
  sendMessage,
  selectWorkspaceFile,
  slugifyWorkspaceName,
  stopRun,
  validateRepositoryForm,
  validateWorkspaceForm,
  type TimelineItem,
  type WorkspaceFile,
} from './workspace-state';

describe('workspace and repository forms', () => {
  it('creates stable workspace slugs', () => {
    expect(slugifyWorkspaceName('  Atlas / Web App  ')).toBe('atlas-web-app');
  });

  it('rejects invalid workspace names', () => {
    expect(validateWorkspaceForm({ name: ' ' })).toBe('Workspace name is required.');
    expect(validateWorkspaceForm({ name: 'A' })).toContain('at least 2');
    expect(validateWorkspaceForm({ name: 'Atlas' })).toBeUndefined();
  });

  it('accepts GitHub URLs and parses the repository identity', () => {
    const url = 'https://github.com/forge/atlas-web';
    expect(validateRepositoryForm({ url })).toBeUndefined();
    expect(parseGithubRepository(url)).toMatchObject({
      fullName: 'forge/atlas-web',
      provider: 'github',
    });
  });

  it('rejects non-GitHub and malformed repository URLs', () => {
    expect(validateRepositoryForm({ url: 'https://gitlab.com/forge/atlas' })).toContain('GitHub');
    expect(parseGithubRepository('not-a-url')).toBeUndefined();
  });

  it('accepts mounted file repositories', () => {
    expect(validateRepositoryForm({ url: 'file:///workspace/repo' })).toBeUndefined();
    expect(parseGithubRepository('file:///workspace/repo')).toMatchObject({ provider: 'local' });
  });
});

const timeline: TimelineItem[] = [
  { id: 'system-1', author: 'Runspace', role: 'system', time: '09:41', body: 'Started' },
];

const files: WorkspaceFile[] = [
  { path: 'src/app/page.tsx', state: 'modified', additions: 4, deletions: 1 },
  { path: 'README.md', state: 'deleted', additions: 0, deletions: 14 },
];

describe('sendMessage', () => {
  it('trims, appends, and clears a non-empty draft', () => {
    const result = sendMessage(timeline, '  keep the terminal open  ', 'human-2', 'now');

    expect(result.sent).toBe(true);
    expect(result.draft).toBe('');
    expect(result.timeline).toEqual([
      ...timeline,
      { id: 'human-2', author: 'You', role: 'human', time: 'now', body: 'keep the terminal open' },
    ]);
    expect(timeline).toHaveLength(1);
  });

  it('does not append whitespace-only drafts', () => {
    const result = sendMessage(timeline, ' \n\t', 'human-2');

    expect(result).toEqual({ timeline, draft: ' \n\t', sent: false });
  });
});

describe('stopRun', () => {
  it('marks a running run completed and appends an audit message', () => {
    const result = stopRun('running', timeline, 'system-2', '10:00');

    expect(result.status).toBe('completed');
    expect(result.timeline.at(-1)).toMatchObject({
      id: 'system-2',
      author: 'Runspace',
      role: 'system',
      time: '10:00',
      tone: 'success',
    });
  });

  it('is idempotent for terminal and queued runs', () => {
    expect(stopRun('completed', timeline, 'ignored')).toEqual({
      status: 'completed',
      timeline,
    });
    expect(stopRun('queued', timeline, 'ignored')).toEqual({ status: 'queued', timeline });
  });
});

describe('selectWorkspaceFile', () => {
  it('returns the file matching the selected path', () => {
    expect(selectWorkspaceFile(files, 'README.md')).toEqual(files[1]);
  });

  it('returns undefined for a stale or unknown selection', () => {
    expect(selectWorkspaceFile(files, 'src/missing.ts')).toBeUndefined();
  });
});
