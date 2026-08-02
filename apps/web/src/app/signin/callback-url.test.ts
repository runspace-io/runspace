import { afterEach, describe, expect, it } from 'vitest';
import { safeCallbackUrl } from './callback-url';

function withLocation(search: string) {
  window.history.replaceState({}, '', `/signin${search}`);
}

afterEach(() => window.history.replaceState({}, '', '/signin'));

describe('safeCallbackUrl', () => {
  it('returns the requested path so deep links survive sign-in', () => {
    withLocation('?callbackUrl=%2Finvite%2Fabc123');
    expect(safeCallbackUrl()).toBe('/invite/abc123');
  });

  it('accepts an absolute URL on this origin by reducing it to a path', () => {
    withLocation(`?callbackUrl=${encodeURIComponent(`${window.location.origin}/invite/abc`)}`);
    expect(safeCallbackUrl()).toBe('/invite/abc');
  });

  // Honouring an off-site value would make sign-in an open redirect.
  it('refuses anything that could leave this origin', () => {
    for (const hostile of [
      'https://evil.example/steal',
      '//evil.example/steal',
      'javascript:alert(1)',
      'http://localhost:9999/',
    ]) {
      withLocation(`?callbackUrl=${encodeURIComponent(hostile)}`);
      expect(safeCallbackUrl(), hostile).toBe('/');
    }
  });

  it('falls back to the workspace root when nothing is requested', () => {
    withLocation('');
    expect(safeCallbackUrl()).toBe('/');
  });
});
