import type { Dispatch, SetStateAction } from 'react';
import type { ApiRun, WorkspaceApiClient } from '@/lib/api-client';

export type RunDispatchInput = {
  api: WorkspaceApiClient;
  workspaceID: string | undefined;
  threadID: string | undefined;
  repositoryID: string | undefined;
  prompt: string;
  setActiveRun: Dispatch<SetStateAction<ApiRun | undefined>>;
  setDraft: (value: string) => void;
  setError: (message: string | undefined) => void;
};

/**
 * Starts a containerised agent run against the channel's resource.
 *
 * Runs are created queued and only execute once started, so both calls belong
 * together — a queued run left unstarted looks like a hang. Setting the active
 * run is what lets streamed `agent.output` events match this channel.
 */
export async function dispatchRun(input: RunDispatchInput): Promise<void> {
  const { api, workspaceID, threadID, repositoryID, setActiveRun, setDraft, setError } = input;
  const prompt = input.prompt.trim();
  if (!prompt || !workspaceID || !threadID || !repositoryID) {
    setError('Connect a resource and open a channel before starting an agent run.');
    return;
  }
  setError(undefined);
  setDraft('');
  try {
    const created = await api.createRun(threadID, {
      runID: nextRunID(),
      workspaceID,
      repositoryID,
      prompt,
    });
    setActiveRun(created);
    setActiveRun(await api.startRun(created.id));
  } catch (reason) {
    setDraft(prompt);
    setError(reason instanceof Error ? reason.message : 'The agent run could not be started.');
  }
}

function nextRunID() {
  return `run_${globalThis.crypto?.randomUUID?.() ?? Date.now().toString(36)}`;
}
