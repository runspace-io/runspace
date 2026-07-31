package persistence

import (
	"context"
	"encoding/json"

	"github.com/runspace/runspace/internal/agentregistry"
)

func (s *Store) UpsertAgentTaskGrant(
	ctx context.Context, grant agentregistry.TaskGrant,
) error {
	permissions, _ := json.Marshal(grant.Permissions)
	_, err := s.db.ExecContext(ctx, `INSERT INTO agent_task_grants
		(task_id,workspace_id,owner_id,agent_id,principal_id,role,permissions,expires_at,created_at,updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (task_id,principal_id) DO UPDATE SET
		role=EXCLUDED.role,permissions=EXCLUDED.permissions,expires_at=EXCLUDED.expires_at,
		updated_at=EXCLUDED.updated_at`,
		grant.TaskID, grant.WorkspaceID, grant.OwnerID, grant.AgentID, grant.PrincipalID,
		grant.Role, permissions, grant.ExpiresAt, grant.CreatedAt, grant.UpdatedAt,
	)
	return err
}

func (s *Store) ListAgentTaskGrants(
	ctx context.Context, taskID, ownerID string,
) ([]agentregistry.TaskGrant, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT task_id,workspace_id,owner_id,agent_id,
		principal_id,role,permissions,expires_at,created_at,updated_at
		FROM agent_task_grants WHERE task_id=$1 AND owner_id=$2 ORDER BY principal_id`,
		taskID, ownerID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var items []agentregistry.TaskGrant
	for rows.Next() {
		var item agentregistry.TaskGrant
		var permissions []byte
		if err := rows.Scan(
			&item.TaskID, &item.WorkspaceID, &item.OwnerID, &item.AgentID,
			&item.PrincipalID, &item.Role, &permissions, &item.ExpiresAt,
			&item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(permissions, &item.Permissions)
		items = append(items, item)
	}
	return items, rows.Err()
}
