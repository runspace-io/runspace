import { describe, expect, it } from 'vitest';
import { localAgentErrorMessage } from './local-agent-error';

describe('localAgentErrorMessage', () => {
  // "New agent chat" is offered on every channel, but a local agent only runs
  // against a resource approved on this device via "Local folder" — a git-URL
  // connect never talks to the Host Agent. Without this mapping the backend's
  // internal wording reached the UI verbatim.
  it('explains an unapproved resource and how to fix it', () => {
    expect(localAgentErrorMessage(new Error('resource is not available to this user'))).toMatch(
      /local folder/i,
    );
  });

  it('explains a missing local runtime', () => {
    expect(localAgentErrorMessage(new Error('local ACP agent is not ready'))).toMatch(
      /host agent is running/i,
    );
  });

  it('falls back to the raw message for anything else', () => {
    expect(localAgentErrorMessage(new Error('network timeout'))).toBe('network timeout');
  });

  it('falls back to a generic message for a non-Error', () => {
    expect(localAgentErrorMessage('nope')).toBe(
      'The local agent could not complete this instruction.',
    );
  });
});
