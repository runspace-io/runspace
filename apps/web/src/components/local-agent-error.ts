/**
 * Maps the Host Agent's raw error strings to something a user can act on.
 *
 * "New agent chat" is offered on every channel regardless of how its resource
 * was connected, but a local agent can only run against a resource approved on
 * this device via "Local folder" — a git-URL connect never talks to the Host
 * Agent. Without this, the backend's internal wording ("resource is not
 * available to this user") reached the UI verbatim.
 */
export function localAgentErrorMessage(reason: unknown): string {
  const message = reason instanceof Error ? reason.message : '';
  if (message.includes('resource is not available')) {
    return (
      "This resource isn't connected on your device yet. Reconnect it as a " +
      'Local folder from the resource menu, then start the chat again.'
    );
  }
  if (message.includes('local ACP agent is not ready')) {
    return (
      'No local agent runtime was found on this device. Make sure the Host ' +
      'Agent is running and the agent (OpenCode, Codex, or Claude) is installed.'
    );
  }
  return message || 'The local agent could not complete this instruction.';
}
