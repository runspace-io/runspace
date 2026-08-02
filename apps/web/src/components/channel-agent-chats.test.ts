import { describe, expect, it } from 'vitest';
import { chatHistorySummary } from './channel-agent-chats';

describe('chatHistorySummary', () => {
  it('reports no chats when both catalogs are empty', () => {
    expect(chatHistorySummary(0, 0)).toBe('No chats yet');
  });

  it('reports only the shared count when there are no local chats', () => {
    expect(chatHistorySummary(2, 0)).toBe('2 shared');
  });

  it('reports only the local count when nothing is shared', () => {
    expect(chatHistorySummary(0, 3)).toBe('3 private');
  });

  it('combines both counts when present', () => {
    expect(chatHistorySummary(2, 3)).toBe('2 shared · 3 private');
  });
});
