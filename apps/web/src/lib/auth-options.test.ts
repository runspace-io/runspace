import { describe, expect, it } from 'vitest';
import { authOptions, authorizeLocalUser } from './auth-options';

describe('auth options', () => {
  it('does not advertise unconfigured identity providers', () => {
    expect(authOptions.providers).toHaveLength(0);
    expect(authOptions.session?.strategy).toBe('jwt');
    expect(authOptions.pages?.signIn).toBe('/signin');
  });

  it('authorizes only explicitly configured local collaborators', () => {
    const users = 'admin:admin,alice:correct-horse';
    expect(authorizeLocalUser('alice', 'correct-horse', users)).toMatchObject({
      id: 'alice',
      name: 'Alice',
    });
    expect(authorizeLocalUser('alice', 'wrong', users)).toBeNull();
    expect(authorizeLocalUser('bob', 'correct-horse', users)).toBeNull();
  });
});
