package agentregistry

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/runspace/runspace/internal/collaboration"
	"github.com/runspace/runspace/internal/workspace"
)

var (
	ErrUnauthorized     = errors.New("authentication required")
	ErrInvalidInput     = errors.New("invalid agent installation")
	ErrQuestionResolved = errors.New("question has already been resolved")
)

type ActivityStatus string

const (
	ActivityStarted         ActivityStatus = "started"
	ActivityCompleted       ActivityStatus = "completed"
	ActivityFailed          ActivityStatus = "failed"
	ActivityCancelled       ActivityStatus = "cancelled"
	ActivityWaitingApproval ActivityStatus = "waiting_approval"
)

type Installation struct {
	OwnerID      string    `json:"owner_id"`
	ID           string    `json:"id"`
	RegistryID   string    `json:"registry_id"`
	Name         string    `json:"name"`
	Description  string    `json:"description,omitempty"`
	Protocol     string    `json:"protocol"`
	Placement    string    `json:"placement"`
	Status       string    `json:"status"`
	Capabilities []string  `json:"capabilities"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Store interface {
	UpsertAgentMetadata(context.Context, string, Installation) error
	ListAgentMetadata(context.Context, string) ([]Installation, error)
	ListWorkspaceAgentMetadata(context.Context, string, string) ([]Installation, error)
}

type Authorizer interface {
	CanRead(context.Context, string, string) error
	ListMembers(context.Context, string, string) ([]workspace.Member, error)
}

type MessageWriter interface {
	CreateAgentMessage(context.Context, string, string, string, string, string) (collaboration.Message, error)
	CreateAgentActivity(context.Context, string, string, string, string, string) (collaboration.Message, error)
}

type GraphProjector interface {
	ProjectAgentTask(
		context.Context, string, string, string, string, string, string,
		string, string, time.Time, time.Time,
	) error
	ProjectTaskArtifact(
		context.Context, string, string, string, string, string, string, time.Time,
	) error
}

type Service struct {
	mu            sync.RWMutex
	items         map[string]map[string]Installation
	store         Store
	grantStore    TaskGrantStore
	taskStore     TaskStore
	messageStore  TaskMessageStore
	questionStore TaskQuestionStore
	answerer      QuestionAnswerer
	publisher     EventPublisher
	authorizer    Authorizer
	messages      MessageWriter
	grants        map[string]map[string]TaskGrant
	tasks         map[string]AgentTask
	executor      TaskExecutor
	graph         GraphProjector
	now           func() time.Time
}

func New(now func() time.Time, authorizer ...Authorizer) *Service {
	service := &Service{
		items:  make(map[string]map[string]Installation),
		grants: make(map[string]map[string]TaskGrant),
		tasks:  make(map[string]AgentTask),
		now:    now,
	}
	if len(authorizer) > 0 {
		service.authorizer = authorizer[0]
	}
	return service
}

func (s *Service) SetStore(store Store)                   { s.store = store }
func (s *Service) SetMessageWriter(writer MessageWriter)  { s.messages = writer }
func (s *Service) SetGraphProjector(graph GraphProjector) { s.graph = graph }

func (s *Service) Upsert(ctx context.Context, userID string, item Installation) (Installation, error) {
	userID = strings.TrimSpace(userID)
	item.ID = strings.TrimSpace(item.ID)
	item.OwnerID = userID
	item.RegistryID = strings.TrimSpace(item.RegistryID)
	item.Name = strings.TrimSpace(item.Name)
	if userID == "" {
		return Installation{}, ErrUnauthorized
	}
	if !strings.HasPrefix(item.ID, "local_agent_") || item.RegistryID == "" || item.Name == "" ||
		item.Protocol != "acp" || item.Placement != "host" {
		return Installation{}, ErrInvalidInput
	}
	item.UpdatedAt = s.now().UTC()
	item.Capabilities = append([]string(nil), item.Capabilities...)
	if s.store != nil {
		if err := s.store.UpsertAgentMetadata(ctx, userID, item); err != nil {
			return Installation{}, err
		}
	}
	s.mu.Lock()
	if s.items[userID] == nil {
		s.items[userID] = make(map[string]Installation)
	}
	s.items[userID][item.ID] = item
	s.mu.Unlock()
	return item, nil
}

func (s *Service) RecordOutput(
	ctx context.Context, userID, workspaceID, threadID, agentID, body string,
) (collaboration.Message, error) {
	items, err := s.List(ctx, userID)
	if err != nil {
		return collaboration.Message{}, err
	}
	owned := false
	for _, item := range items {
		if item.ID == agentID && item.OwnerID == userID {
			owned = true
			break
		}
	}
	if !owned || s.messages == nil || strings.TrimSpace(body) == "" {
		return collaboration.Message{}, ErrInvalidInput
	}
	return s.messages.CreateAgentMessage(ctx, userID, agentID, workspaceID, threadID, body)
}

// RecordActivity publishes only server-authored status copy. Callers cannot
// smuggle private prompts, model output, paths, or terminal content into chat.
func (s *Service) RecordActivity(
	ctx context.Context, userID, workspaceID, threadID, agentID string, status ActivityStatus,
) (collaboration.Message, error) {
	if !s.owns(ctx, userID, agentID) || s.messages == nil {
		return collaboration.Message{}, ErrInvalidInput
	}
	body, ok := activityBody(status)
	if !ok {
		return collaboration.Message{}, ErrInvalidInput
	}
	return s.messages.CreateAgentActivity(ctx, userID, agentID, workspaceID, threadID, body)
}

func (s *Service) owns(ctx context.Context, userID, agentID string) bool {
	items, err := s.List(ctx, userID)
	if err != nil {
		return false
	}
	for _, item := range items {
		if item.ID == agentID && item.OwnerID == userID {
			return true
		}
	}
	return false
}

func activityBody(status ActivityStatus) (string, bool) {
	switch status {
	case ActivityStarted:
		return "Started work in a private agent chat.", true
	case ActivityCompleted:
		return "Agent chat work completed. Results remain private until shared.", true
	case ActivityFailed:
		return "The private agent chat failed. Details remain on the owner’s device.", true
	case ActivityCancelled:
		return "The private agent chat was cancelled.", true
	case ActivityWaitingApproval:
		return "Waiting for the owner to approve a capability.", true
	default:
		return "", false
	}
}

func (s *Service) Directory(ctx context.Context, userID, workspaceID string) ([]Installation, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, ErrUnauthorized
	}
	if s.authorizer != nil {
		if err := s.authorizer.CanRead(ctx, workspaceID, userID); err != nil {
			return nil, err
		}
	}
	if s.store != nil {
		items, err := s.store.ListWorkspaceAgentMetadata(ctx, userID, workspaceID)
		return s.withPresence(items), err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Installation
	if s.authorizer == nil {
		for _, owned := range s.items {
			for _, item := range owned {
				out = append(out, item)
			}
		}
		return s.withPresence(out), nil
	}
	members, err := s.authorizer.ListMembers(ctx, userID, workspaceID)
	if err != nil {
		return nil, err
	}
	for _, member := range members {
		owned := s.items[member.UserID]
		for _, item := range owned {
			out = append(out, item)
		}
	}
	return s.withPresence(out), nil
}

func (s *Service) List(ctx context.Context, userID string) ([]Installation, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, ErrUnauthorized
	}
	if s.store != nil {
		items, err := s.store.ListAgentMetadata(ctx, userID)
		return s.withPresence(items), err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Installation, 0, len(s.items[userID]))
	for _, item := range s.items[userID] {
		item.Capabilities = append([]string(nil), item.Capabilities...)
		out = append(out, item)
	}
	return s.withPresence(out), nil
}

func (s *Service) withPresence(items []Installation) []Installation {
	cutoff := s.now().UTC().Add(-45 * time.Second)
	for index := range items {
		if items[index].UpdatedAt.Before(cutoff) {
			items[index].Status = "offline"
		}
	}
	return items
}
