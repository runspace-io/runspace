import { describe, expect, it, vi } from 'vitest';
import type { ApiRun, WorkspaceApiClient } from '@/lib/api-client';
import { dispatchRun } from './run-dispatch';

function run(overrides: Partial<ApiRun> = {}): ApiRun {
  return {
    id: 'run-1',
    workspace_id: 'workspace-1',
    prompt: 'Fix the failing test',
    status: 'queued',
    ...overrides,
  };
}

function harness(api: Partial<WorkspaceApiClient>) {
  const setActiveRun = vi.fn();
  const setDraft = vi.fn();
  const setError = vi.fn();
  return {
    setActiveRun,
    setDraft,
    setError,
    input: {
      api: api as WorkspaceApiClient,
      workspaceID: 'workspace-1',
      threadID: 'thread-1',
      repositoryID: 'repo-1',
      prompt: '  Fix the failing test  ',
      setActiveRun,
      setDraft,
      setError,
    },
  };
}

describe('dispatchRun', () => {
  // A created run sits queued until started, so creating without starting
  // would look to the user like the agent hung.
  it('creates and starts the run, then tracks it as active', async () => {
    const createRun = vi.fn().mockResolvedValue(run());
    const startRun = vi.fn().mockResolvedValue(run({ status: 'running' }));
    const { input, setActiveRun, setDraft, setError } = harness({ createRun, startRun });

    await dispatchRun(input);

    expect(createRun).toHaveBeenCalledWith(
      'thread-1',
      expect.objectContaining({
        workspaceID: 'workspace-1',
        repositoryID: 'repo-1',
        prompt: 'Fix the failing test',
      }),
    );
    expect(startRun).toHaveBeenCalledWith('run-1');
    expect(setActiveRun).toHaveBeenLastCalledWith(expect.objectContaining({ status: 'running' }));
    expect(setDraft).toHaveBeenCalledWith('');
    expect(setError).toHaveBeenCalledWith(undefined);
  });

  it('restores the draft when the run cannot start', async () => {
    const createRun = vi.fn().mockResolvedValue(run());
    const startRun = vi.fn().mockRejectedValue(new Error('agent image missing'));
    const { input, setDraft, setError } = harness({ createRun, startRun });

    await dispatchRun(input);

    expect(setDraft).toHaveBeenLastCalledWith('Fix the failing test');
    expect(setError).toHaveBeenLastCalledWith('agent image missing');
  });

  it('refuses to dispatch without a resource and never calls the API', async () => {
    const createRun = vi.fn();
    const { input, setError } = harness({ createRun });

    await dispatchRun({ ...input, repositoryID: undefined });

    expect(createRun).not.toHaveBeenCalled();
    expect(setError).toHaveBeenCalledWith(expect.stringContaining('Connect a resource'));
  });

  it('ignores a blank prompt', async () => {
    const createRun = vi.fn();
    const { input } = harness({ createRun });

    await dispatchRun({ ...input, prompt: '   ' });

    expect(createRun).not.toHaveBeenCalled();
  });
});
