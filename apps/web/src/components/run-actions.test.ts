import { describe, expect, it } from 'vitest';
import { setRunMonotonic } from './run-state';
import type { ApiRun } from '../lib/api-types';

const run = (status: ApiRun['status']): ApiRun => ({
  id: 'run-1',
  workspace_id: 'workspace-1',
  prompt: 'test',
  status,
});

describe('run action state updates', () => {
  it('does not downgrade a finished run when start resolves late', () => {
    let current: ApiRun | undefined = run('succeeded');
    setRunMonotonic((next) => {
      current = typeof next === 'function' ? next(current) : next;
    }, run('running'));
    expect(current?.status).toBe('succeeded');
  });

  it('accepts a terminal response for an active run', () => {
    let current: ApiRun | undefined = run('running');
    setRunMonotonic((next) => {
      current = typeof next === 'function' ? next(current) : next;
    }, run('cancelled'));
    expect(current?.status).toBe('cancelled');
  });
});
