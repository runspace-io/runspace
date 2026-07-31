import { describe, expect, it } from 'vitest';
import { findMention } from './use-mention-picker';

describe('findMention', () => {
  it('finds a member query at the caret', () => {
    expect(findMention('Hello @nah', 10)).toEqual({ start: 6, query: 'nah' });
  });

  it('supports mentions after opening punctuation', () => {
    expect(findMention('Ask (@alice', 11)).toEqual({ start: 5, query: 'alice' });
  });

  it('does not treat an email address as a mention', () => {
    expect(findMention('mail@example.com', 16)).toBeUndefined();
  });

  it('closes the query after whitespace', () => {
    expect(findMention('Hello @nahid ', 13)).toBeUndefined();
  });
});
