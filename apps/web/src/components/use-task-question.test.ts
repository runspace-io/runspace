import { describe, expect, it } from 'vitest';
import type { ApiTaskQuestion } from '@/lib/api-client';
import { answerError, fromLocal, fromRemote } from './use-task-question';

function remoteQuestion(status: ApiTaskQuestion['status']): ApiTaskQuestion {
  return {
    id: 'q_1_7',
    task_id: 'local_session_1',
    title: 'Run rm -rf build/',
    options: [
      { id: 'once', name: 'Allow once', kind: 'allow_once' },
      { id: 'reject', name: 'Reject', kind: 'reject_once' },
    ],
    status,
    asked_at: '2026-08-01T10:00:00Z',
    updated_at: '2026-08-01T10:00:00Z',
  };
}

describe('fromRemote', () => {
  it('surfaces an open question with its options intact', () => {
    expect(fromRemote(remoteQuestion('open'))).toEqual({
      id: 'q_1_7',
      title: 'Run rm -rf build/',
      options: [
        { id: 'once', name: 'Allow once', kind: 'allow_once' },
        { id: 'reject', name: 'Reject', kind: 'reject_once' },
      ],
    });
  });

  // Offering a resolved question would let someone answer twice, or try to
  // unblock an agent that already moved on.
  it('ignores questions that are no longer open', () => {
    expect(fromRemote(remoteQuestion('answered'))).toBeUndefined();
    expect(fromRemote(remoteQuestion('cancelled'))).toBeUndefined();
    expect(fromRemote(undefined)).toBeUndefined();
  });
});

describe('fromLocal', () => {
  it('maps the owner’s own pending question', () => {
    expect(
      fromLocal({
        id: 'q_1_7',
        title: 'Run rm -rf build/',
        options: [{ id: 'once', name: 'Allow once', kind: 'allow_once' }],
        asked_at: '2026-08-01T10:00:00Z',
      }),
    ).toMatchObject({ id: 'q_1_7', title: 'Run rm -rf build/' });
    expect(fromLocal(undefined)).toBeUndefined();
  });
});

describe('answerError', () => {
  // Two people can hit the button at once; the loser needs a plain explanation
  // rather than a raw conflict from the API.
  it('explains a question that someone else already resolved', () => {
    expect(answerError(new Error('question has already been resolved'))).toBe(
      'This question was already answered.',
    );
    expect(answerError(new Error('permission request is no longer pending'))).toBe(
      'This question was already answered.',
    );
  });

  it('passes other failures through and falls back for non-errors', () => {
    expect(answerError(new Error('host agent unreachable'))).toBe('host agent unreachable');
    expect(answerError('nope')).toBe('The answer could not be delivered to the agent.');
    expect(answerError(new Error(''))).toBe('The answer could not be delivered to the agent.');
  });
});
