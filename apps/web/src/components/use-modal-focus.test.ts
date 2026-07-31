import { describe, expect, it } from 'vitest';
import { focusTargetIndex } from './use-modal-focus';

describe('focusTargetIndex', () => {
  it('moves forward and wraps at the end', () => {
    expect(focusTargetIndex(0, 3, false)).toBe(1);
    expect(focusTargetIndex(2, 3, false)).toBe(0);
  });

  it('moves backward and wraps at the start', () => {
    expect(focusTargetIndex(2, 3, true)).toBe(1);
    expect(focusTargetIndex(0, 3, true)).toBe(2);
  });

  it('enters the trap predictably when focus starts outside', () => {
    expect(focusTargetIndex(-1, 3, false)).toBe(0);
    expect(focusTargetIndex(-1, 3, true)).toBe(2);
  });
});
