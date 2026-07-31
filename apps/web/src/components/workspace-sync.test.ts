import { describe, expect, it } from 'vitest';
import { initialWorkspace } from './workspace-sync';
import type { WorkspaceSummary } from '@/lib/workspace-state';

const existing = {
  id: 'existing',
  name: 'Existing',
  slug: 'existing',
  resourceCount: 0,
} satisfies WorkspaceSummary;
const created = {
  id: 'created',
  name: 'Created',
  slug: 'created',
  resourceCount: 0,
} satisfies WorkspaceSummary;

describe('initialWorkspace', () => {
  it('selects the first workspace when no selection exists', () => {
    expect(initialWorkspace(undefined, [existing])).toEqual(existing);
  });

  it('does not overwrite a workspace selected while the list was loading', () => {
    expect(initialWorkspace(created, [existing])).toEqual(created);
  });
});
