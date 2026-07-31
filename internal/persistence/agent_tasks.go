package persistence

import (
	"context"
	"database/sql"
	"errors"

	"github.com/runspace/runspace/internal/agentregistry"
)

func (s *Store) UpsertAgentTask(ctx context.Context, task agentregistry.AgentTask) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO agent_tasks
		(id,workspace_id,thread_id,owner_id,agent_id,resource_id,title,status,created_at,updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (id) DO UPDATE SET title=EXCLUDED.title,status=EXCLUDED.status,
		resource_id=EXCLUDED.resource_id,updated_at=EXCLUDED.updated_at
		WHERE agent_tasks.owner_id=EXCLUDED.owner_id`,
		task.ID, task.WorkspaceID, task.ThreadID, task.OwnerID, task.AgentID,
		task.ResourceID, task.Title, task.Status, task.CreatedAt, task.UpdatedAt,
	)
	return err
}

func (s *Store) ListAgentTasks(
	ctx context.Context, workspaceID, threadID, principalID string,
) ([]agentregistry.AgentTask, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT t.id,t.workspace_id,t.thread_id,t.owner_id,
		t.agent_id,t.resource_id,t.title,t.status,t.updated_at,t.created_at
		FROM agent_tasks t
		LEFT JOIN agent_task_grants g ON g.task_id=t.id AND g.principal_id=$3
			AND (g.expires_at IS NULL OR g.expires_at > NOW())
		WHERE t.workspace_id=$1 AND t.thread_id=$2 AND (t.owner_id=$3 OR g.principal_id=$3)
		ORDER BY t.updated_at DESC`, workspaceID, threadID, principalID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	tasks, err := scanAgentTasks(rows)
	if err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tasks, nil
}

func (s *Store) GetAgentTaskAccess(
	ctx context.Context, taskID, principalID string,
) (agentregistry.AgentTask, []string, error) {
	var task agentregistry.AgentTask
	var permissions []byte
	err := s.db.QueryRowContext(ctx, `SELECT t.id,t.workspace_id,t.thread_id,t.owner_id,
		t.agent_id,t.resource_id,t.title,t.status,t.updated_at,t.created_at,
		CASE WHEN t.owner_id=$2 THEN '["task.view","task.contribute","task.control","artifact.share"]'::jsonb
			ELSE COALESCE(g.permissions,'[]'::jsonb) END
		FROM agent_tasks t
		LEFT JOIN agent_task_grants g ON g.task_id=t.id AND g.principal_id=$2
			AND (g.expires_at IS NULL OR g.expires_at > NOW())
		WHERE t.id=$1 AND (t.owner_id=$2 OR g.principal_id=$2)`,
		taskID, principalID,
	).Scan(
		&task.ID, &task.WorkspaceID, &task.ThreadID, &task.OwnerID, &task.AgentID,
		&task.ResourceID, &task.Title, &task.Status, &task.UpdatedAt, &task.CreatedAt,
		&permissions,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return agentregistry.AgentTask{}, nil, agentregistry.ErrTaskUnavailable
	}
	return task, parseStringSlice(permissions), err
}

type rowScanner interface {
	Next() bool
	Scan(...any) error
	Err() error
}

func scanAgentTasks(rows rowScanner) ([]agentregistry.AgentTask, error) {
	var tasks []agentregistry.AgentTask
	for rows.Next() {
		var task agentregistry.AgentTask
		if err := rows.Scan(
			&task.ID, &task.WorkspaceID, &task.ThreadID, &task.OwnerID, &task.AgentID,
			&task.ResourceID, &task.Title, &task.Status, &task.UpdatedAt, &task.CreatedAt,
		); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}
