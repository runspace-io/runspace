import { describe, expect, it } from 'vitest';
import { eventToTimelineItem } from './api-client';
import { agentTaskEvent } from './api-normalizers';

function taskEvent(overrides: Record<string, unknown>) {
  return {
    id: 'event-1',
    type: 'agent.task.message',
    workspace_id: 'atlas',
    actor_id: 'local_agent_abc',
    actor_type: 'agent',
    occurred_at: '2025-01-01T10:00:00Z',
    payload: {},
    ...overrides,
  };
}

describe('agentTaskEvent', () => {
  it('reads task identity without routing the event into the channel timeline', () => {
    const event = taskEvent({
      thread_id: 'thread-1',
      payload: { task_id: 'local_session_1', message_id: 'm1', role: 'agent' },
    });

    expect(agentTaskEvent(event)).toMatchObject({
      taskID: 'local_session_1',
      threadID: 'thread-1',
    });
    // Private agent chats stay out of the shared timeline until shared.
    expect(eventToTimelineItem(event)).toBeUndefined();
  });

  it('reads status from status events', () => {
    expect(
      agentTaskEvent(
        taskEvent({
          type: 'agent.task.status',
          payload: { task_id: 'local_session_1', status: 'completed' },
        }),
      ),
    ).toMatchObject({ taskID: 'local_session_1', status: 'completed' });
  });

  it('ignores unrelated events and events without a task', () => {
    expect(agentTaskEvent(taskEvent({ type: 'agent.output' }))).toBeUndefined();
    expect(agentTaskEvent(taskEvent({ payload: {} }))).toBeUndefined();
    expect(agentTaskEvent(taskEvent({ payload: null }))).toBeUndefined();
  });

  it('reads question events without exposing what the agent asked', () => {
    const asked = taskEvent({
      type: 'agent.question.asked',
      payload: { task_id: 'local_session_1', question_id: 'q_1_7' },
    });

    expect(agentTaskEvent(asked)).toMatchObject({
      taskID: 'local_session_1',
      questionID: 'q_1_7',
    });
    expect(eventToTimelineItem(asked)).toBeUndefined();
    expect(
      agentTaskEvent(
        taskEvent({
          type: 'agent.question.answered',
          payload: { task_id: 'local_session_1', question_id: 'q_1_7', answered_by: 'nahid' },
        }),
      ),
    ).toMatchObject({ questionID: 'q_1_7' });
  });
});
