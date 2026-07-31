package events

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/runspace/runspace/internal/contracts"
)

func testEvent(id string) contracts.EventEnvelope {
	event, err := contracts.NewEvent(id, "message.created", "workspace-1", "user-1", "user", map[string]string{"body": "hello"}, time.Unix(1, 0))
	if err != nil {
		panic(err)
	}
	return event
}

func TestMemoryStoreAppendIsIdempotentAndPublishes(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()
	event := testEvent("event-1")
	if err := store.Append(ctx, event); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(store.Append(ctx, event), ErrDuplicate) {
		t.Fatal("expected duplicate event rejection")
	}
	pending, err := store.Pending(ctx, 10)
	if err != nil || len(pending) != 1 {
		t.Fatalf("pending=%v err=%v", pending, err)
	}
	if err := store.MarkPublished(ctx, event.ID); err != nil {
		t.Fatal(err)
	}
	pending, err = store.Pending(ctx, 10)
	if err != nil || len(pending) != 0 {
		t.Fatalf("pending after publish=%v err=%v", pending, err)
	}
}
