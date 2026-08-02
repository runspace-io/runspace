package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/runspace/runspace/internal/agentregistry"
)

// taskQuestionSchema keeps parked permission questions on the server so a
// browser reload — or a different person holding the grant — can still see and
// answer what the agent is blocked on.
const taskQuestionSchema = `
CREATE TABLE IF NOT EXISTS agent_task_questions (id text NOT NULL, task_id text NOT NULL REFERENCES agent_tasks(id) ON DELETE CASCADE, title text NOT NULL, options jsonb NOT NULL DEFAULT '[]'::jsonb, status text NOT NULL, answered_by text NOT NULL DEFAULT '', answered_option text NOT NULL DEFAULT '', asked_at timestamptz NOT NULL, updated_at timestamptz NOT NULL, PRIMARY KEY(task_id,id));
CREATE INDEX IF NOT EXISTS agent_task_questions_open ON agent_task_questions(task_id,status);`

func (s *Store) UpsertAgentTaskQuestion(
	ctx context.Context, question agentregistry.TaskQuestion,
) error {
	options, err := json.Marshal(question.Options)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO agent_task_questions
		(id,task_id,title,options,status,answered_by,answered_option,asked_at,updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (task_id,id) DO UPDATE SET status=EXCLUDED.status,
		answered_by=EXCLUDED.answered_by,answered_option=EXCLUDED.answered_option,
		updated_at=EXCLUDED.updated_at`,
		question.ID, question.TaskID, question.Title, options, question.Status,
		question.AnsweredBy, question.AnsweredOption, question.AskedAt, question.UpdatedAt,
	)
	return err
}

func (s *Store) ListAgentTaskQuestions(
	ctx context.Context, taskID string,
) ([]agentregistry.TaskQuestion, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,task_id,title,options,status,
		answered_by,answered_option,asked_at,updated_at
		FROM agent_task_questions WHERE task_id=$1 ORDER BY asked_at,id`, taskID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	questions := make([]agentregistry.TaskQuestion, 0)
	for rows.Next() {
		question, err := scanQuestion(rows.Scan)
		if err != nil {
			return nil, err
		}
		questions = append(questions, question)
	}
	return questions, rows.Err()
}

func (s *Store) GetAgentTaskQuestion(
	ctx context.Context, taskID, questionID string,
) (agentregistry.TaskQuestion, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,task_id,title,options,status,
		answered_by,answered_option,asked_at,updated_at
		FROM agent_task_questions WHERE task_id=$1 AND id=$2`, taskID, questionID)
	question, err := scanQuestion(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return agentregistry.TaskQuestion{}, agentregistry.ErrTaskUnavailable
	}
	return question, err
}

// CancelOpenAgentTaskQuestions closes anything still waiting once a turn has
// moved on, so the UI never offers an answer the agent can no longer accept.
func (s *Store) CancelOpenAgentTaskQuestions(
	ctx context.Context, taskID string, at time.Time,
) error {
	_, err := s.db.ExecContext(ctx, `UPDATE agent_task_questions
		SET status=$3, updated_at=$2 WHERE task_id=$1 AND status=$4`,
		taskID, at, agentregistry.QuestionCancelled, agentregistry.QuestionOpen,
	)
	return err
}

func scanQuestion(
	scan func(...any) error,
) (agentregistry.TaskQuestion, error) {
	var question agentregistry.TaskQuestion
	var options []byte
	if err := scan(
		&question.ID, &question.TaskID, &question.Title, &options, &question.Status,
		&question.AnsweredBy, &question.AnsweredOption, &question.AskedAt, &question.UpdatedAt,
	); err != nil {
		return agentregistry.TaskQuestion{}, err
	}
	if len(options) > 0 {
		_ = json.Unmarshal(options, &question.Options)
	}
	return question, nil
}
