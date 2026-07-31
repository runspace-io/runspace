package agentregistry

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/runspace/runspace/internal/collaboration"
)

var ErrTaskUnavailable = errors.New("agent task is unavailable")

type AgentTask struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	ThreadID    string    `json:"thread_id"`
	OwnerID     string    `json:"owner_id"`
	AgentID     string    `json:"agent_id"`
	ResourceID  string    `json:"resource_id"`
	Title       string    `json:"title"`
	Status      string    `json:"status"`
	UpdatedAt   time.Time `json:"updated_at"`
	CreatedAt   time.Time `json:"created_at"`
}

type TaskOutput struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

type TaskStore interface {
	UpsertAgentTask(context.Context, AgentTask) error
	ListAgentTasks(context.Context, string, string, string) ([]AgentTask, error)
	GetAgentTaskAccess(context.Context, string, string) (AgentTask, []string, error)
}

type TaskExecutor interface {
	Prompt(context.Context, AgentTask, string) ([]TaskOutput, error)
	Cancel(context.Context, AgentTask) error
}

func (s *Service) SetTaskStore(store TaskStore)          { s.taskStore = store }
func (s *Service) SetTaskExecutor(executor TaskExecutor) { s.executor = executor }

func (s *Service) UpsertTask(
	ctx context.Context, callerID string, task AgentTask,
) (AgentTask, error) {
	callerID = strings.TrimSpace(callerID)
	task.ID = strings.TrimSpace(task.ID)
	task.WorkspaceID = strings.TrimSpace(task.WorkspaceID)
	task.ThreadID = strings.TrimSpace(task.ThreadID)
	task.AgentID = strings.TrimSpace(task.AgentID)
	task.ResourceID = strings.TrimSpace(task.ResourceID)
	task.Title = strings.TrimSpace(task.Title)
	if callerID == "" {
		return AgentTask{}, ErrUnauthorized
	}
	if !strings.HasPrefix(task.ID, "local_session_") || task.WorkspaceID == "" ||
		task.ThreadID == "" || task.ResourceID == "" || task.Title == "" ||
		!s.owns(ctx, callerID, task.AgentID) {
		return AgentTask{}, ErrInvalidInput
	}
	if s.authorizer != nil {
		if err := s.authorizer.CanRead(ctx, task.WorkspaceID, callerID); err != nil {
			return AgentTask{}, err
		}
	}
	now := s.now().UTC()
	task.OwnerID = callerID
	task.Status = normalizeTaskStatus(task.Status)
	task.UpdatedAt = now
	s.mu.Lock()
	if existing, ok := s.tasks[task.ID]; ok {
		task.CreatedAt = existing.CreatedAt
	} else {
		task.CreatedAt = now
	}
	s.tasks[task.ID] = task
	s.mu.Unlock()
	if s.taskStore != nil {
		if err := s.taskStore.UpsertAgentTask(ctx, task); err != nil {
			return AgentTask{}, err
		}
	}
	s.projectTask(ctx, task)
	return task, nil
}

func (s *Service) ListTasks(
	ctx context.Context, callerID, workspaceID, threadID string,
) ([]AgentTask, error) {
	if strings.TrimSpace(callerID) == "" {
		return nil, ErrUnauthorized
	}
	if workspaceID == "" || threadID == "" {
		return nil, ErrInvalidInput
	}
	if s.authorizer != nil {
		if err := s.authorizer.CanRead(ctx, workspaceID, callerID); err != nil {
			return nil, err
		}
	}
	if s.taskStore != nil {
		return s.taskStore.ListAgentTasks(ctx, workspaceID, threadID, callerID)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var items []AgentTask
	for _, task := range s.tasks {
		if task.WorkspaceID == workspaceID && task.ThreadID == threadID &&
			(task.OwnerID == callerID || s.hasGrantLocked(task.ID, callerID, "task.view")) {
			items = append(items, task)
		}
	}
	return items, nil
}

func (s *Service) InputTask(
	ctx context.Context, callerID, taskID, input string,
) ([]TaskOutput, error) {
	task, permissions, err := s.taskAccess(ctx, callerID, taskID)
	if err != nil || (!contains(permissions, "task.contribute") && task.OwnerID != callerID) {
		return nil, ErrTaskUnavailable
	}
	if strings.TrimSpace(input) == "" || s.executor == nil {
		return nil, ErrInvalidInput
	}
	s.persistTaskStatus(ctx, task, "running")
	_, _ = s.recordTaskActivity(ctx, callerID, task, ActivityStarted)
	outputs, err := s.executor.Prompt(ctx, task, input)
	status := ActivityCompleted
	taskStatus := "completed"
	if err != nil {
		status = ActivityFailed
		taskStatus = "failed"
	}
	s.persistTaskStatus(ctx, task, taskStatus)
	_, _ = s.recordTaskActivity(ctx, callerID, task, status)
	return outputs, err
}

func (s *Service) CancelTask(ctx context.Context, callerID, taskID string) error {
	task, permissions, err := s.taskAccess(ctx, callerID, taskID)
	if err != nil || (!contains(permissions, "task.control") && task.OwnerID != callerID) {
		return ErrTaskUnavailable
	}
	if s.executor == nil {
		return ErrInvalidInput
	}
	if err := s.executor.Cancel(ctx, task); err != nil {
		return err
	}
	s.persistTaskStatus(ctx, task, "cancelled")
	_, _ = s.recordTaskActivity(ctx, callerID, task, ActivityCancelled)
	return nil
}

func (s *Service) ShareTaskArtifact(
	ctx context.Context, callerID, taskID, body string,
) (collaboration.Message, error) {
	task, permissions, err := s.taskAccess(ctx, callerID, taskID)
	if err != nil || (!contains(permissions, "artifact.share") && task.OwnerID != callerID) ||
		s.messages == nil || strings.TrimSpace(body) == "" {
		return collaboration.Message{}, ErrTaskUnavailable
	}
	message, err := s.messages.CreateAgentMessage(
		ctx, callerID, task.AgentID, task.WorkspaceID, task.ThreadID, body,
	)
	if err != nil {
		return collaboration.Message{}, err
	}
	if s.graph != nil {
		_ = s.graph.ProjectTaskArtifact(
			ctx, task.ID, message.ID, task.WorkspaceID, task.ThreadID,
			callerID, artifactTitle(body), message.CreatedAt,
		)
	}
	return message, nil
}

func (s *Service) taskAccess(
	ctx context.Context, callerID, taskID string,
) (AgentTask, []string, error) {
	if s.taskStore != nil {
		return s.taskStore.GetAgentTaskAccess(ctx, taskID, callerID)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	task, ok := s.tasks[taskID]
	if !ok {
		return AgentTask{}, nil, ErrTaskUnavailable
	}
	if task.OwnerID == callerID {
		return task, []string{"task.view", "task.contribute", "task.control", "artifact.share"}, nil
	}
	grant := s.grants[taskID][callerID]
	if grant.ExpiresAt != nil && grant.ExpiresAt.Before(s.now().UTC()) {
		return AgentTask{}, nil, ErrTaskUnavailable
	}
	return task, append([]string(nil), grant.Permissions...), nil
}

func (s *Service) recordTaskActivity(
	ctx context.Context, callerID string, task AgentTask, status ActivityStatus,
) (collaboration.Message, error) {
	body, ok := activityBody(status)
	if !ok || s.messages == nil {
		return collaboration.Message{}, ErrInvalidInput
	}
	return s.messages.CreateAgentActivity(
		ctx, callerID, task.AgentID, task.WorkspaceID, task.ThreadID, body,
	)
}

func (s *Service) hasGrantLocked(taskID, principalID, permission string) bool {
	grant := s.grants[taskID][principalID]
	return (grant.ExpiresAt == nil || grant.ExpiresAt.After(s.now().UTC())) &&
		contains(grant.Permissions, permission)
}

func (s *Service) persistTaskStatus(ctx context.Context, task AgentTask, status string) {
	task.Status = status
	task.UpdatedAt = s.now().UTC()
	s.mu.Lock()
	s.tasks[task.ID] = task
	s.mu.Unlock()
	if s.taskStore != nil {
		_ = s.taskStore.UpsertAgentTask(ctx, task)
	}
	s.projectTask(ctx, task)
}

func (s *Service) projectTask(ctx context.Context, task AgentTask) {
	if s.graph == nil {
		return
	}
	_ = s.graph.ProjectAgentTask(
		ctx, task.ID, task.WorkspaceID, task.ThreadID, task.OwnerID, task.AgentID,
		task.ResourceID, task.Title, task.Status, task.CreatedAt, task.UpdatedAt,
	)
}

func artifactTitle(body string) string {
	title := strings.TrimSpace(body)
	const limit = 72
	runes := []rune(title)
	if len(runes) > limit {
		title = string(runes[:limit-1]) + "…"
	}
	return title
}

func contains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func normalizeTaskStatus(status string) string {
	switch strings.TrimSpace(status) {
	case "running", "completed", "failed", "cancelled":
		return status
	default:
		return "ready"
	}
}
