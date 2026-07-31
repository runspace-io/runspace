package collaboration

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/runspace/runspace/internal/contracts"
	"github.com/runspace/runspace/internal/events"
)

var (
	ErrUnauthorized = errors.New("collaboration authorization required")
	ErrNotFound     = errors.New("collaboration resource not found")
	ErrInvalid      = errors.New("invalid collaboration input")
)

type Authorizer interface {
	CanRead(context.Context, string, string) error
	CanWrite(context.Context, string, string) error
}

type Thread struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	ChannelID   string    `json:"channel_id,omitempty"`
	Title       string    `json:"title"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
}

// Channel is a workspace conversation. A child channel inherits repository
// and configuration values from its nearest configured ancestor.
type Channel struct {
	ID            string         `json:"id"`
	WorkspaceID   string         `json:"workspace_id"`
	Name          string         `json:"name"`
	ParentID      string         `json:"parent_id,omitempty"`
	RepositoryID  string         `json:"repository_id,omitempty"`
	RepositoryIDs []string       `json:"repository_ids,omitempty"`
	Config        map[string]any `json:"config,omitempty"`
	CreatedBy     string         `json:"created_by"`
	CreatedAt     time.Time      `json:"created_at"`
}

type Message struct {
	ID        string    `json:"id"`
	ThreadID  string    `json:"thread_id"`
	ActorID   string    `json:"actor_id"`
	ActorType string    `json:"actor_type"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

type Service interface {
	CreateChannel(context.Context, string, string, string, string, string, map[string]any) (Channel, error)
	CreateChannelWithRepositories(context.Context, string, string, string, string, []string, map[string]any) (Channel, error)
	UpdateChannel(context.Context, string, string, string, ChannelPatch) (Channel, error)
	ListChannels(context.Context, string, string) ([]Channel, error)
	GetChannel(context.Context, string, string) (Channel, error)
	CreateThread(context.Context, string, string, string, ...string) (Thread, error)
	ListThreads(context.Context, string, string) ([]Thread, error)
	CreateMessage(context.Context, string, string, string, string, string) (Message, error)
	ListMessages(context.Context, string, string, string) ([]Message, error)
}

// Store is the durable chat boundary. PostgreSQL implementations may be
// attached to MemoryService for write-through and restart-safe reads.
type Store interface {
	CreateThread(context.Context, Thread) error
	ListThreads(context.Context, string, string) ([]Thread, error)
	CreateMessage(context.Context, Message) error
	ListMessages(context.Context, string, string, string) ([]Message, error)
}

type ChannelStore interface {
	CreateCollaborationChannel(context.Context, Channel) error
	UpdateCollaborationChannel(context.Context, Channel) error
	ListCollaborationChannels(context.Context, string, string) ([]Channel, error)
}

type MemoryService struct {
	mu         sync.RWMutex
	clock      func() time.Time
	seq        uint64
	authorizer Authorizer
	publisher  events.Publisher
	store      Store
	threads    map[string]Thread
	messages   map[string][]Message
	channels   map[string]Channel
	graph      GraphProjector
}

func (s *MemoryService) SetStore(store Store) { s.mu.Lock(); defer s.mu.Unlock(); s.store = store }
func (s *MemoryService) SetGraphProjector(graph GraphProjector) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.graph = graph
}

type GraphProjector interface {
	ProjectDiscussion(context.Context, string, string, string, string, string, time.Time) error
}

// SetPublisher wires the service to the durable outbox/NATS publisher. It is
// intentionally explicit so tests can use a recording publisher and production
// wiring can require NATS without coupling domain code to a client library.
func (s *MemoryService) SetPublisher(publisher events.Publisher) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.publisher = publisher
}

func NewMemoryService(clock func() time.Time, authorizer Authorizer) *MemoryService {
	if clock == nil {
		clock = time.Now
	}
	return &MemoryService{clock: clock, authorizer: authorizer, threads: make(map[string]Thread), messages: make(map[string][]Message), channels: make(map[string]Channel)}
}

func (s *MemoryService) id(prefix string) string {
	s.seq++
	return prefix + "_" + time.Now().UTC().Format("20060102150405") + "_" + string(rune('a'+s.seq%26))
}

func (s *MemoryService) CreateThread(ctx context.Context, userID, workspaceID, title string, channelIDs ...string) (Thread, error) {
	if err := s.validateThreadCreate(ctx, userID, workspaceID, title); err != nil {
		return Thread{}, err
	}
	channelID, err := s.resolveThreadChannel(ctx, userID, workspaceID, channelIDs)
	if err != nil {
		return Thread{}, err
	}
	s.mu.Lock()
	now := s.clock().UTC()
	thread := Thread{ID: s.id("thread"), WorkspaceID: workspaceID, ChannelID: channelID, Title: strings.TrimSpace(title), CreatedBy: userID, CreatedAt: now}
	s.threads[thread.ID] = thread
	store := s.store
	s.mu.Unlock()
	if err := s.persistThread(ctx, store, thread); err != nil {
		return Thread{}, err
	}
	s.mu.RLock()
	graph := s.graph
	s.mu.RUnlock()
	if graph != nil {
		_ = graph.ProjectDiscussion(
			ctx, thread.ID, thread.WorkspaceID, thread.ChannelID,
			thread.CreatedBy, thread.Title, thread.CreatedAt,
		)
	}
	return thread, nil
}

func (s *MemoryService) validateThreadCreate(ctx context.Context, userID, workspaceID, title string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if invalidThreadInput(userID, workspaceID, title) {
		return ErrInvalid
	}
	if s.authorizer == nil {
		return ErrUnauthorized
	}
	return s.authorizer.CanWrite(ctx, workspaceID, userID)
}

func invalidThreadInput(userID, workspaceID, title string) bool {
	return strings.TrimSpace(userID) == "" || strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(title) == ""
}

func (s *MemoryService) resolveThreadChannel(ctx context.Context, userID, workspaceID string, channelIDs []string) (string, error) {
	if len(channelIDs) == 0 {
		return "", nil
	}
	channelID := strings.TrimSpace(channelIDs[0])
	if channelID == "" {
		return "", nil
	}
	channel, err := s.GetChannel(ctx, userID, channelID)
	if err != nil {
		return "", err
	}
	if channel.WorkspaceID != workspaceID {
		return "", ErrInvalid
	}
	return channelID, nil
}

func (s *MemoryService) persistThread(ctx context.Context, store Store, thread Thread) error {
	if store != nil {
		if err := store.CreateThread(ctx, thread); err != nil {
			return err
		}
	}
	return s.publish(ctx, "thread.created", thread.WorkspaceID, thread.CreatedBy, "user", thread)
}

func (s *MemoryService) ListThreads(ctx context.Context, userID, workspaceID string) ([]Thread, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.authorizer == nil {
		return nil, ErrUnauthorized
	}
	if err := s.authorizer.CanRead(ctx, workspaceID, userID); err != nil {
		return nil, err
	}
	s.mu.RLock()
	store := s.store
	defer s.mu.RUnlock()
	if store != nil {
		if persisted, err := store.ListThreads(ctx, userID, workspaceID); err == nil && persisted != nil {
			return persisted, nil
		}
	}
	result := make([]Thread, 0)
	for _, thread := range s.threads {
		if thread.WorkspaceID == workspaceID {
			result = append(result, thread)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.Before(result[j].CreatedAt) })
	return result, nil
}

func (s *MemoryService) publish(ctx context.Context, eventType, workspaceID, actorID, actorType string, payload any) error {
	s.mu.Lock()
	publisher := s.publisher
	eventID := s.id("event")
	now := s.clock()
	s.mu.Unlock()
	if publisher == nil {
		return nil
	}
	event, err := contracts.NewEvent(eventID, eventType, workspaceID, actorID, actorType, payload, now)
	if err != nil {
		return err
	}
	return publisher.Publish(ctx, event)
}
