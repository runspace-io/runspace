import type { Dispatch, SetStateAction } from 'react';
import type { ApiChannel, WorkspaceApiClient } from '@/lib/api-client';
import type { TimelineItem, WorkspaceSummary } from '@/lib/workspace-state';
import { sendMessageRequest } from './channel-actions';
import { channelHasAgent } from './run-actions';

export function useChannelComposer(input: {
  api: WorkspaceApiClient;
  workspace: WorkspaceSummary | undefined;
  channel: ApiChannel | undefined;
  threadID: string | undefined;
  draft: string;
  setDraft: (value: string) => void;
  setTimeline: Dispatch<SetStateAction<TimelineItem[]>>;
  setError: (error: string | undefined) => void;
}) {
  const { api, workspace, channel, threadID, draft, setDraft, setTimeline, setError } = input;
  const agentAvailable = channelHasAgent(channel?.config);
  const send = () =>
    sendMessageRequest({
      api,
      workspace,
      threadID,
      draft,
      setDraft,
      setTimeline,
      setError,
    });
  return {
    agentAvailable,
    send,
  };
}
