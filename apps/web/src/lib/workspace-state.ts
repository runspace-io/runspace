export type RunStatus = 'running' | 'completed' | 'queued';
export type FileState = 'modified' | 'added' | 'deleted';

export type WorkspaceFile = {
  path: string;
  state: FileState;
  additions: number;
  deletions: number;
};

export type WorkspaceTreeEntry = {
  path: string;
  kind: 'file' | 'directory';
};

export type TimelineItem = {
  id: string;
  author: string;
  role: 'human' | 'agent' | 'system';
  time: string;
  body: string;
  provider?: string;
  providerID?: string;
  activity?: boolean;
  tone?: 'muted' | 'success';
};

export type WorkspaceSummary = {
  id: string;
  name: string;
  slug: string;
  resourceCount: number;
  repositoryCount?: number;
};

export type ResourceSummary = {
  id: string;
  fullName: string;
  cloneURL?: string;
  defaultBranch: string;
  provider: 'github' | 'local' | 'mirror' | 'folder';
};
export type RepositorySummary = ResourceSummary;

export type WorkspaceForm = { name: string };
export type RepositoryForm = { url: string };

export type SendMessageResult = {
  timeline: TimelineItem[];
  draft: string;
  sent: boolean;
};

export function slugifyWorkspaceName(name: string): string {
  return name
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-|-$/g, '')
    .slice(0, 48);
}

export function validateWorkspaceForm(form: WorkspaceForm): string | undefined {
  const name = form.name.trim();
  if (!name) return 'Workspace name is required.';
  if (name.length < 2) return 'Workspace name must be at least 2 characters.';
  if (name.length > 64) return 'Workspace name must be 64 characters or fewer.';
  if (!slugifyWorkspaceName(name)) return 'Workspace name must include a letter or number.';
  return undefined;
}

export function validateRepositoryForm(form: RepositoryForm): string | undefined {
  const value = form.url.trim();
  if (!value) return 'Resource URL is required.';
  if (value.startsWith('file://') || value.startsWith('local:')) return undefined;
  try {
    const url = new URL(value);
    if (url.protocol !== 'https:' || url.hostname !== 'github.com') {
      return 'Use an HTTPS GitHub repository URL.';
    }
    const parts = url.pathname.split('/').filter(Boolean);
    if (parts.length !== 2) return 'Use a repository URL like github.com/org/repository.';
  } catch {
    return 'Enter a valid resource URL.';
  }
  return undefined;
}

export function parseGithubRepository(url: string): RepositorySummary | undefined {
  if (validateRepositoryForm({ url })) return undefined;
  if (url.trim().startsWith('file://') || url.trim().startsWith('local:')) {
    const path = url.trim().replace(/^local:/, '');
    return {
      id: `local/${path.split('/').filter(Boolean).at(-1) ?? 'repository'}`,
      fullName: path,
      cloneURL: path.startsWith('file://') ? path : `file://${path}`,
      defaultBranch: 'main',
      provider: 'local',
    };
  }
  const parts = new URL(url.trim()).pathname.split('/').filter(Boolean);
  const fullName = `${parts[0]}/${parts[1]}`;
  return { id: fullName, fullName, defaultBranch: 'main', provider: 'github' };
}

/** Applies the same trimming and reset rules used by the composer. */
export function sendMessage(
  timeline: readonly TimelineItem[],
  draft: string,
  id: string,
  time = 'now',
): SendMessageResult {
  const body = draft.trim();
  if (!body) return { timeline: [...timeline], draft, sent: false };

  return {
    timeline: [...timeline, { id, author: 'You', role: 'human', time, body }],
    draft: '',
    sent: true,
  };
}

/** Stops an active run and records an auditable system message. */
export function stopRun(
  status: RunStatus,
  timeline: readonly TimelineItem[],
  id: string,
  time = 'now',
): { status: RunStatus; timeline: TimelineItem[] } {
  if (status !== 'running') return { status, timeline: [...timeline] };

  return {
    status: 'completed',
    timeline: [
      ...timeline,
      {
        id,
        author: 'Runspace',
        role: 'system',
        time,
        body: 'Run stopped by you. The checkout is preserved for review.',
        tone: 'success',
      },
    ],
  };
}

export function selectWorkspaceFile(
  files: readonly WorkspaceFile[],
  path: string,
): WorkspaceFile | undefined {
  return files.find((file) => file.path === path);
}
