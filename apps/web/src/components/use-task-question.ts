'use client';

import { useCallback, useEffect, useState } from 'react';
import type { ApiTaskQuestion, WorkspaceApiClient } from '@/lib/api-client';
import {
  answerLocalAgentQuestion,
  type LocalAgentSession,
  type LocalPendingQuestion,
} from '@/lib/host-agent-client';

export type PendingQuestion = {
  id: string;
  title: string;
  options: Array<{ id: string; name: string; kind: string }>;
};

type QuestionInput = {
  api: WorkspaceApiClient;
  agentID: string;
  threadID: string;
  resourceID: string;
  session: LocalAgentSession | undefined;
  remote: boolean;
  /** Bumped by realtime task events so a grantee sees questions as they arrive. */
  revision: number;
  onAnswered: () => void;
};

/**
 * Surfaces the question an agent is blocked on.
 *
 * The owner reads it straight off their own session; anyone else reads the
 * gateway copy, which is also the only path that can carry their grant.
 */
export function useTaskQuestion(input: QuestionInput) {
  const { api, session, remote, revision } = input;
  const taskID = session?.id;
  const [remoteQuestion, setRemoteQuestion] = useState<ApiTaskQuestion>();
  const [remoteCanAnswer, setRemoteCanAnswer] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();

  useEffect(() => {
    if (!remote || !taskID) {
      setRemoteQuestion(undefined);
      return;
    }
    let active = true;
    void api
      .listTaskQuestions(taskID)
      .then(({ questions, canAnswer }) => {
        if (!active) return;
        setRemoteQuestion(questions.find((item) => item.status === 'open'));
        setRemoteCanAnswer(canAnswer);
      })
      .catch(() => active && setRemoteQuestion(undefined));
    return () => {
      active = false;
    };
  }, [api, remote, taskID, revision]);

  const question = remote ? fromRemote(remoteQuestion) : fromLocal(session?.question);
  // The owner is answering their own agent on their own device.
  const canAnswer = remote ? remoteCanAnswer : true;

  const answer = useCallback(
    async (optionID: string) => {
      if (!question || !taskID || busy) return;
      setBusy(true);
      setError(undefined);
      try {
        if (remote) await api.answerTaskQuestion(taskID, question.id, optionID);
        else
          await answerLocalAgentQuestion({
            userID: api.actorID,
            agentID: input.agentID,
            resourceID: input.resourceID,
            threadID: input.threadID,
            taskID,
            questionID: question.id,
            optionID,
          });
        setRemoteQuestion(undefined);
        input.onAnswered();
      } catch (reason) {
        setError(answerError(reason));
      } finally {
        setBusy(false);
      }
    },
    [api, busy, input, question, remote, taskID],
  );

  return { question, canAnswer, busy, error, answer };
}

/**
 * Picks the open question from a server list. Answered and cancelled ones are
 * history: offering them would let a viewer resolve something twice, or unblock
 * an agent that has already moved on.
 */
export function fromRemote(question: ApiTaskQuestion | undefined): PendingQuestion | undefined {
  if (!question || question.status !== 'open') return undefined;
  return { id: question.id, title: question.title, options: question.options };
}

export function fromLocal(question: LocalPendingQuestion | undefined): PendingQuestion | undefined {
  if (!question) return undefined;
  return { id: question.id, title: question.title, options: question.options };
}

export function answerError(reason: unknown) {
  const message = reason instanceof Error ? reason.message : '';
  // 409 means someone else answered first, or the agent moved on.
  if (message.includes('resolved') || message.includes('no longer pending')) {
    return 'This question was already answered.';
  }
  return message || 'The answer could not be delivered to the agent.';
}
