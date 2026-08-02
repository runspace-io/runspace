package persistence

import (
	"context"

	"github.com/runspace/runspace/internal/agentregistry"
)

// taskMessageSchema keeps host-executed agent turns on the server so grantees
// and post-refresh clients read the same transcript the owner saw live.
const taskMessageSchema = `
CREATE TABLE IF NOT EXISTS agent_task_messages (id text PRIMARY KEY, task_id text NOT NULL REFERENCES agent_tasks(id) ON DELETE CASCADE, role text NOT NULL, body text NOT NULL, created_at timestamptz NOT NULL);
CREATE INDEX IF NOT EXISTS agent_task_messages_task ON agent_task_messages(task_id,created_at);`

// AppendAgentTaskMessages is idempotent on message ID so a host agent that
// retries a push after a network error cannot duplicate the transcript.
func (s *Store) AppendAgentTaskMessages(
	ctx context.Context, taskID string, messages []agentregistry.TaskMessage,
) error {
	if len(messages) == 0 {
		return nil
	}
	transaction, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = transaction.Rollback() }()
	for _, message := range messages {
		if _, err := transaction.ExecContext(ctx, `INSERT INTO agent_task_messages
			(id,task_id,role,body,created_at) VALUES ($1,$2,$3,$4,$5)
			ON CONFLICT (id) DO NOTHING`,
			message.ID, taskID, message.Role, message.Body, message.CreatedAt,
		); err != nil {
			return err
		}
	}
	return transaction.Commit()
}

func (s *Store) ListAgentTaskMessages(
	ctx context.Context, taskID string,
) ([]agentregistry.TaskMessage, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,role,body,created_at
		FROM agent_task_messages WHERE task_id=$1 ORDER BY created_at,id`, taskID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	messages := make([]agentregistry.TaskMessage, 0)
	for rows.Next() {
		var message agentregistry.TaskMessage
		if err := rows.Scan(
			&message.ID, &message.Role, &message.Body, &message.CreatedAt,
		); err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	return messages, rows.Err()
}
