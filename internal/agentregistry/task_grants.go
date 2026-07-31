package agentregistry

import (
	"context"
	"strings"
	"time"
)

type TaskGrantStore interface {
	UpsertAgentTaskGrant(context.Context, TaskGrant) error
	ListAgentTaskGrants(context.Context, string, string) ([]TaskGrant, error)
}

type TaskGrant struct {
	TaskID      string     `json:"task_id"`
	WorkspaceID string     `json:"workspace_id"`
	OwnerID     string     `json:"owner_id"`
	AgentID     string     `json:"agent_id"`
	PrincipalID string     `json:"principal_id"`
	Role        string     `json:"role"`
	Permissions []string   `json:"permissions"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (s *Service) SetTaskGrantStore(store TaskGrantStore) { s.grantStore = store }

func (s *Service) GrantTaskAccess(
	ctx context.Context, callerID string, grant TaskGrant,
) (TaskGrant, error) {
	callerID = strings.TrimSpace(callerID)
	grant.TaskID = strings.TrimSpace(grant.TaskID)
	grant.WorkspaceID = strings.TrimSpace(grant.WorkspaceID)
	grant.AgentID = strings.TrimSpace(grant.AgentID)
	grant.PrincipalID = strings.TrimSpace(grant.PrincipalID)
	grant.OwnerID = callerID
	permissions, validRole := taskRolePermissions(grant.Role)
	if callerID == "" {
		return TaskGrant{}, ErrUnauthorized
	}
	if grant.TaskID == "" || grant.WorkspaceID == "" || grant.PrincipalID == "" ||
		!validRole || !s.owns(ctx, callerID, grant.AgentID) {
		return TaskGrant{}, ErrInvalidInput
	}
	task, _, err := s.taskAccess(ctx, callerID, grant.TaskID)
	if err != nil || task.OwnerID != callerID || task.WorkspaceID != grant.WorkspaceID ||
		task.AgentID != grant.AgentID {
		return TaskGrant{}, ErrInvalidInput
	}
	if err := s.validateTaskPrincipal(ctx, callerID, grant); err != nil {
		return TaskGrant{}, err
	}
	now := s.now().UTC()
	grant.Role = strings.TrimSpace(grant.Role)
	grant.Permissions = permissions
	grant.CreatedAt = now
	grant.UpdatedAt = now
	if s.grantStore != nil {
		if err := s.grantStore.UpsertAgentTaskGrant(ctx, grant); err != nil {
			return TaskGrant{}, err
		}
	}
	s.rememberTaskGrant(grant)
	return grant, nil
}

func (s *Service) validateTaskPrincipal(
	ctx context.Context, callerID string, grant TaskGrant,
) error {
	if s.authorizer == nil {
		return nil
	}
	if err := s.authorizer.CanRead(ctx, grant.WorkspaceID, callerID); err != nil {
		return err
	}
	members, err := s.authorizer.ListMembers(ctx, callerID, grant.WorkspaceID)
	if err != nil {
		return err
	}
	for _, item := range members {
		if item.UserID == grant.PrincipalID {
			return nil
		}
	}
	return ErrInvalidInput
}

func (s *Service) rememberTaskGrant(grant TaskGrant) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.grants[grant.TaskID] == nil {
		s.grants[grant.TaskID] = make(map[string]TaskGrant)
	}
	if existing, ok := s.grants[grant.TaskID][grant.PrincipalID]; ok {
		grant.CreatedAt = existing.CreatedAt
	}
	s.grants[grant.TaskID][grant.PrincipalID] = grant
}

func (s *Service) ListTaskGrants(
	ctx context.Context, callerID, workspaceID, taskID, agentID string,
) ([]TaskGrant, error) {
	if strings.TrimSpace(callerID) == "" {
		return nil, ErrUnauthorized
	}
	if taskID == "" || workspaceID == "" || !s.owns(ctx, callerID, agentID) {
		return nil, ErrInvalidInput
	}
	if s.authorizer != nil {
		if err := s.authorizer.CanRead(ctx, workspaceID, callerID); err != nil {
			return nil, err
		}
	}
	if s.grantStore != nil {
		return s.grantStore.ListAgentTaskGrants(ctx, taskID, callerID)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]TaskGrant, 0, len(s.grants[taskID]))
	for _, item := range s.grants[taskID] {
		if item.OwnerID == callerID && item.WorkspaceID == workspaceID {
			item.Permissions = append([]string(nil), item.Permissions...)
			items = append(items, item)
		}
	}
	return items, nil
}

func taskRolePermissions(role string) ([]string, bool) {
	switch strings.TrimSpace(role) {
	case "viewer":
		return []string{"task.view", "artifact.view"}, true
	case "contributor":
		return []string{"task.view", "task.contribute", "artifact.view", "artifact.share"}, true
	case "operator":
		return []string{
			"task.view", "task.contribute", "task.control", "artifact.view", "artifact.share",
		}, true
	case "approver":
		return []string{"task.view", "task.approve", "artifact.view"}, true
	default:
		return nil, false
	}
}
