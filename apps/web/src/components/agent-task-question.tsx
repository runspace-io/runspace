import { ShieldQuestion } from 'lucide-react';
import type { PendingQuestion } from './use-task-question';

/**
 * The agent is stopped until this is answered. Rendered for anyone who can see
 * the task; the buttons are only live for someone allowed to answer, so a
 * viewer understands why nothing is moving without being able to move it.
 */
export function AgentTaskQuestion({
  question,
  canAnswer,
  busy,
  error,
  onAnswer,
}: {
  question: PendingQuestion;
  canAnswer: boolean;
  busy: boolean;
  error: string | undefined;
  onAnswer: (optionID: string) => void;
}) {
  return (
    <section className="agent-task-question" aria-labelledby="agent-task-question-title">
      <header>
        <span className="agent-task-question-mark">
          <ShieldQuestion size={16} />
        </span>
        <div>
          <p className="eyebrow">WAITING FOR APPROVAL</p>
          <h3 id="agent-task-question-title">{question.title}</h3>
        </div>
      </header>
      {canAnswer ? (
        <div className="agent-task-question-options">
          {question.options.map((option) => (
            <button
              key={option.id}
              type="button"
              className={optionClass(option.kind)}
              disabled={busy}
              onClick={() => onAnswer(option.id)}
            >
              {option.name || option.id}
            </button>
          ))}
        </div>
      ) : (
        <p className="agent-task-question-readonly">
          The agent is waiting on its owner or an approver.
        </p>
      )}
      {error && (
        <p className="agent-task-error" role="alert">
          {error}
        </p>
      )}
    </section>
  );
}

function optionClass(kind: string) {
  return kind.startsWith('allow') ? 'dialog-primary' : 'dialog-secondary';
}
