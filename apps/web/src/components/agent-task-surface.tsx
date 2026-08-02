'use client';

import {
  AgentActivityLine,
  AgentTaskComposer,
  AgentTaskHeader,
  AgentTaskMeta,
  TaskLog,
} from './agent-task-parts';
import { type AgentTaskProps, useAgentTask } from './agent-task-controller';
import { AgentTaskQuestion } from './agent-task-question';
import { TaskAccessPanel } from './task-access-panel';
import { useTaskQuestion } from './use-task-question';

export function AgentTaskSurface(props: AgentTaskProps) {
  const task = useAgentTask(props);
  const question = useTaskQuestion({
    api: props.api,
    agentID: props.agentID,
    threadID: props.threadID,
    resourceID: task.resourceID,
    session: task.session,
    remote: task.remote,
    revision: props.taskRevision ?? 0,
    onAnswered: () => task.refresh(),
  });
  return (
    <section className="agent-task-surface" aria-labelledby="agent-task-title">
      <AgentTaskHeader
        title={task.title}
        status={surfaceStatus(task.busy, task.session?.status)}
        accessOpen={task.accessOpen}
        accessAvailable={accessAvailable(task.remote, props.registered)}
        onAccessChange={() => task.setAccessOpen((current) => !current)}
        onClose={props.onClose}
        closeLabel="Back to channel"
      />
      <AgentTaskMeta
        resources={props.resources}
        resourceID={task.resourceID}
        provider={task.selectedResource?.provider}
        busy={task.busy}
        onResourceChange={task.setResourceID}
      />
      <TaskLog
        messages={task.session?.messages ?? []}
        shared={task.shared}
        onShare={(message) => void task.share(message)}
      />
      <AgentActivityLine activity={task.activity} />
      {question.question && (
        <AgentTaskQuestion
          question={question.question}
          canAnswer={question.canAnswer}
          busy={question.busy}
          error={question.error}
          onAnswer={(optionID) => void question.answer(optionID)}
        />
      )}
      <ChatSurfaceState props={props} task={task} />
      <AgentTaskComposer
        session={task.session}
        instruction={task.instruction}
        resourceID={task.resourceID}
        busy={task.busy}
        onInstructionChange={task.setInstruction}
        onRun={() => void task.run()}
        onCancel={() => void task.cancel()}
      />
    </section>
  );
}

function ChatSurfaceState({
  props,
  task,
}: {
  props: AgentTaskProps;
  task: ReturnType<typeof useAgentTask>;
}) {
  return (
    <>
      {task.accessOpen && task.session && (
        <TaskAccessPanel
          api={props.api}
          workspaceID={props.workspaceID}
          taskID={task.session.id}
          agentID={props.agentID}
          onClose={() => task.setAccessOpen(false)}
        />
      )}
      {task.error && (
        <p className="agent-task-error" role="alert">
          {task.error}
        </p>
      )}
    </>
  );
}

function surfaceStatus(busy: boolean, status: string | undefined) {
  return busy ? 'running' : (status ?? 'draft');
}

function accessAvailable(remote: boolean, registered: boolean | undefined) {
  return !remote && registered !== false;
}
