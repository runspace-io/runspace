package agentregistry

import (
	"context"
	"strings"
	"time"

	"github.com/runspace/runspace/internal/contracts"
)

type TaskMessage struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

// TaskStreamUpdate is one push from a Host Agent as a turn unfolds. It carries
// enough task identity to create the task row on the first chunk, so a turn is
// never dropped because the browser had not registered the task yet.
type TaskStreamUpdate struct {
	WorkspaceID string        `json:"workspace_id"`
	ThreadID    string        `json:"thread_id"`
	AgentID     string        `json:"agent_id"`
	ResourceID  string        `json:"resource_id"`
	Title       string        `json:"title"`
	Status      string        `json:"status"`
	Messages    []TaskMessage `json:"messages"`
	Question    *TaskQuestion `json:"question,omitempty"`
}

type TaskMessageStore interface {
	AppendAgentTaskMessages(context.Context, string, []TaskMessage) error
	ListAgentTaskMessages(context.Context, string) ([]TaskMessage, error)
}

type EventPublisher interface {
	Publish(context.Context, contracts.EventEnvelope) error
}

func (s *Service) SetTaskMessageStore(store TaskMessageStore) { s.messageStore = store }
func (s *Service) SetEventPublisher(publisher EventPublisher) { s.publisher = publisher }

// RecordTaskStream persists a Host Agent push and announces it on the bus.
//
// Bodies are deliberately left out of the published event: the realtime hub is
// workspace-scoped, so anything published there reaches every member. Listeners
// learn that a task advanced and re-read the transcript through ListTaskMessages,
// which enforces the per-task grant.
func (s *Service) RecordTaskStream(
	ctx context.Context, callerID, taskID string, update TaskStreamUpdate,
) error {
	task, err := s.UpsertTask(ctx, callerID, AgentTask{
		ID: taskID, WorkspaceID: update.WorkspaceID, ThreadID: update.ThreadID,
		AgentID: update.AgentID, ResourceID: update.ResourceID,
		Title: update.Title, Status: update.Status,
	})
	if err != nil {
		return err
	}
	stored := validTaskMessages(update.Messages, s.now().UTC())
	if len(stored) > 0 && s.messageStore != nil {
		if err := s.messageStore.AppendAgentTaskMessages(ctx, task.ID, stored); err != nil {
			return err
		}
	}
	for _, message := range stored {
		s.publishTaskEvent(ctx, task, contracts.EventAgentTaskMessage, map[string]any{
			"task_id": task.ID, "thread_id": task.ThreadID, "owner_id": task.OwnerID,
			"agent_id": task.AgentID, "message_id": message.ID, "role": message.Role,
		}, message.CreatedAt)
	}
	s.recordTaskQuestion(ctx, task, normalizeQuestion(update.Question))
	s.publishTaskEvent(ctx, task, contracts.EventAgentTaskStatus, map[string]any{
		"task_id": task.ID, "thread_id": task.ThreadID, "owner_id": task.OwnerID,
		"agent_id": task.AgentID, "status": task.Status,
	}, task.UpdatedAt)
	return nil
}

func (s *Service) ListTaskMessages(
	ctx context.Context, callerID, taskID string,
) ([]TaskMessage, error) {
	task, permissions, err := s.taskAccess(ctx, callerID, taskID)
	if err != nil || (!contains(permissions, "task.view") && task.OwnerID != callerID) {
		return nil, ErrTaskUnavailable
	}
	if s.messageStore == nil {
		return []TaskMessage{}, nil
	}
	return s.messageStore.ListAgentTaskMessages(ctx, task.ID)
}

func (s *Service) publishTaskEvent(
	ctx context.Context, task AgentTask, eventType string, payload map[string]any, at time.Time,
) {
	if s.publisher == nil {
		return
	}
	id := task.ID + "-" + eventType + "-" + at.Format(time.RFC3339Nano)
	if messageID, ok := payload["message_id"].(string); ok {
		id = messageID + "-" + eventType
	}
	event, err := contracts.NewEvent(
		id, eventType, task.WorkspaceID, task.AgentID, "agent", payload, at,
	)
	if err == nil {
		_ = s.publisher.Publish(ctx, event)
	}
}

func validTaskMessages(messages []TaskMessage, fallback time.Time) []TaskMessage {
	stored := make([]TaskMessage, 0, len(messages))
	for _, message := range messages {
		message.ID = strings.TrimSpace(message.ID)
		message.Body = strings.TrimSpace(message.Body)
		if message.ID == "" || message.Body == "" ||
			(message.Role != "user" && message.Role != "agent") {
			continue
		}
		if message.CreatedAt.IsZero() {
			message.CreatedAt = fallback
		}
		stored = append(stored, message)
	}
	return stored
}
