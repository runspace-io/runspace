package events

import (
	"context"
	"errors"
	"sync"

	"github.com/runspace/runspace/internal/contracts"
)

var (
	ErrInvalidEvent = errors.New("event is invalid")
	ErrDuplicate    = errors.New("event already exists")
)

// Store is the persistence boundary used by command handlers and publishers.
// PostgreSQL/JetStream can implement it without changing domain code.
type Store interface {
	Append(context.Context, contracts.EventEnvelope) error
	Pending(context.Context, int) ([]contracts.EventEnvelope, error)
	MarkPublished(context.Context, string) error
}

type memoryRecord struct {
	event     contracts.EventEnvelope
	published bool
}

// MemoryStore is deterministic and concurrency-safe for unit tests and local
// development. It deliberately models append-only/idempotent semantics.
type MemoryStore struct {
	mu      sync.RWMutex
	order   []string
	records map[string]memoryRecord
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{records: make(map[string]memoryRecord)}
}

func (s *MemoryStore) Append(ctx context.Context, event contracts.EventEnvelope) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := event.Validate(); err != nil {
		return errors.Join(ErrInvalidEvent, err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.records[event.ID]; exists {
		return ErrDuplicate
	}
	s.records[event.ID] = memoryRecord{event: event}
	s.order = append(s.order, event.ID)
	return nil
}

func (s *MemoryStore) Pending(ctx context.Context, limit int) ([]contracts.EventEnvelope, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limit < 1 {
		return []contracts.EventEnvelope{}, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]contracts.EventEnvelope, 0, limit)
	for _, id := range s.order {
		record := s.records[id]
		if record.published {
			continue
		}
		result = append(result, record.event)
		if len(result) == limit {
			break
		}
	}
	return result, nil
}

func (s *MemoryStore) MarkPublished(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	record, exists := s.records[id]
	if !exists {
		return errors.New("event not found")
	}
	record.published = true
	s.records[id] = record
	return nil
}
