package realtime

import (
	"context"
	"sync"

	"github.com/runspace/runspace/internal/contracts"
)

// Hub is the transport-neutral realtime boundary. WebSocket and NATS adapters
// can subscribe to it; domain services only publish durable event facts.
type Hub interface {
	Subscribe(context.Context, string) (<-chan contracts.EventEnvelope, func(), error)
	SubscribeSince(context.Context, string, string) (<-chan contracts.EventEnvelope, func(), error)
	Publish(context.Context, contracts.EventEnvelope) error
}

type HistorySink interface {
	AddHistory([]contracts.EventEnvelope)
}

type subscriber struct {
	workspaceID string
	channel     chan contracts.EventEnvelope
}

type MemoryHub struct {
	mu          sync.RWMutex
	subscribers map[string][]subscriber
	history     map[string][]contracts.EventEnvelope
}

func NewMemoryHub() *MemoryHub {
	return &MemoryHub{subscribers: make(map[string][]subscriber), history: make(map[string][]contracts.EventEnvelope)}
}

func (h *MemoryHub) AddHistory(events []contracts.EventEnvelope) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, event := range events {
		if !containsEvent(h.history[event.WorkspaceID], event.ID) {
			h.history[event.WorkspaceID] = append(h.history[event.WorkspaceID], event)
		}
	}
}

func (h *MemoryHub) Subscribe(ctx context.Context, workspaceID string) (<-chan contracts.EventEnvelope, func(), error) {
	return h.SubscribeSince(ctx, workspaceID, "")
}

func (h *MemoryHub) SubscribeSince(ctx context.Context, workspaceID, lastEventID string) (<-chan contracts.EventEnvelope, func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, func() {}, err
	}
	h.mu.Lock()
	backlog := replayAfter(h.history[workspaceID], lastEventID)
	channel := make(chan contracts.EventEnvelope, len(backlog)+32)
	s := subscriber{workspaceID: workspaceID, channel: channel}
	for _, event := range backlog {
		channel <- event
	}
	h.subscribers[workspaceID] = append(h.subscribers[workspaceID], s)
	h.mu.Unlock()
	var once sync.Once
	closeSubscription := func() {
		once.Do(func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			items := h.subscribers[workspaceID]
			for index, item := range items {
				if item.channel != channel {
					continue
				}
				h.subscribers[workspaceID] = append(items[:index], items[index+1:]...)
				close(channel)
				return
			}
		})
	}
	return channel, closeSubscription, nil
}

func (h *MemoryHub) Publish(ctx context.Context, event contracts.EventEnvelope) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := event.Validate(); err != nil {
		return err
	}
	h.mu.Lock()
	if !containsEvent(h.history[event.WorkspaceID], event.ID) {
		h.history[event.WorkspaceID] = append(h.history[event.WorkspaceID], event)
	}
	defer h.mu.Unlock()
	for _, item := range h.subscribers[event.WorkspaceID] {
		select {
		case item.channel <- event:
		default:
			// A slow client must not block domain publishers. It will recover from
			// durable state when it reconnects.
		}
	}
	return nil
}

func replayAfter(history []contracts.EventEnvelope, lastID string) []contracts.EventEnvelope {
	if lastID == "" {
		return append([]contracts.EventEnvelope(nil), history...)
	}
	for index, event := range history {
		if event.ID == lastID {
			return append([]contracts.EventEnvelope(nil), history[index+1:]...)
		}
	}
	return append([]contracts.EventEnvelope(nil), history...)
}

func containsEvent(history []contracts.EventEnvelope, id string) bool {
	for _, event := range history {
		if event.ID == id {
			return true
		}
	}
	return false
}
