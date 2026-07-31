package collaboration

import (
	"context"
	"errors"
	"testing"
	"time"
)

type allowAll struct{}

func (allowAll) CanRead(context.Context, string, string) error  { return nil }
func (allowAll) CanWrite(context.Context, string, string) error { return nil }

func TestMemoryCollaborationLifecycle(t *testing.T) {
	clock := func() time.Time { return time.Unix(1, 0) }
	service := NewMemoryService(clock, allowAll{})
	thread, err := service.CreateThread(context.Background(), "alice", "workspace-1", "Implement status")
	if err != nil {
		t.Fatal(err)
	}
	message, err := service.CreateMessage(context.Background(), "alice", "workspace-1", thread.ID, "user", "Please keep the terminal visible")
	if err != nil {
		t.Fatal(err)
	}
	messages, err := service.ListMessages(context.Background(), "alice", "workspace-1", thread.ID)
	if err != nil || len(messages) != 1 || messages[0].ID != message.ID {
		t.Fatalf("messages=%v err=%v", messages, err)
	}
	if _, err := service.CreateThread(context.Background(), "", "workspace-1", "bad"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected invalid user, got %v", err)
	}
}

func TestListMessagesReturnsNonNilEmptySlice(t *testing.T) {
	service := NewMemoryService(time.Now, allowAll{})
	thread, err := service.CreateThread(context.Background(), "alice", "workspace-1", "empty")
	if err != nil {
		t.Fatal(err)
	}
	messages, err := service.ListMessages(context.Background(), "alice", "workspace-1", thread.ID)
	if err != nil {
		t.Fatal(err)
	}
	if messages == nil || len(messages) != 0 {
		t.Fatalf("messages=%#v, want non-nil empty slice", messages)
	}
}

func TestChannelInheritance(t *testing.T) {
	service := NewMemoryService(func() time.Time { return time.Unix(1, 0) }, allowAll{})
	parent, err := service.CreateChannel(context.Background(), "alice", "workspace-1", "engineering", "", "repo-1", map[string]any{"agent": "acp", "model": "fast"})
	if err != nil {
		t.Fatal(err)
	}
	child, err := service.CreateChannel(context.Background(), "alice", "workspace-1", "backend", parent.ID, "", map[string]any{"model": "smart"})
	if err != nil {
		t.Fatal(err)
	}
	if child.RepositoryID != "repo-1" || child.Config["agent"] != "acp" || child.Config["model"] != "smart" {
		t.Fatalf("unexpected inheritance: %+v", child)
	}
	if _, err := service.CreateChannel(context.Background(), "alice", "workspace-1", "bad", "missing", "", nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected missing parent, got %v", err)
	}
}

func TestChannelUpdatePreservesParentInheritance(t *testing.T) {
	service := NewMemoryService(time.Now, allowAll{})
	parent, err := service.CreateChannel(context.Background(), "alice", "workspace-1", "engineering", "", "repo-1", map[string]any{"agent": "acp", "model": "fast"})
	if err != nil {
		t.Fatal(err)
	}
	child, err := service.CreateChannel(context.Background(), "alice", "workspace-1", "backend", parent.ID, "", map[string]any{"model": "smart"})
	if err != nil {
		t.Fatal(err)
	}
	name := "api"
	updated, err := service.UpdateChannel(context.Background(), "alice", "workspace-1", child.ID, ChannelPatch{Name: &name, Config: map[string]any{"model": "balanced"}})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != name || updated.RepositoryID != "repo-1" || updated.Config["agent"] != "acp" || updated.Config["model"] != "balanced" {
		t.Fatalf("unexpected update: %+v", updated)
	}
}

func TestChannelScopedThreadAndMessage(t *testing.T) {
	service := NewMemoryService(time.Now, allowAll{})
	channel, err := service.CreateChannel(context.Background(), "alice", "workspace-1", "backend", "", "repo-1", map[string]any{"agent": "acp"})
	if err != nil {
		t.Fatal(err)
	}
	thread, err := service.CreateThread(context.Background(), "alice", "workspace-1", "backend chat", channel.ID)
	if err != nil {
		t.Fatal(err)
	}
	if thread.ChannelID != channel.ID {
		t.Fatalf("thread channel = %q, want %q", thread.ChannelID, channel.ID)
	}
	if _, err := service.CreateThread(context.Background(), "alice", "workspace-2", "wrong workspace", channel.ID); !errors.Is(err, ErrInvalid) {
		t.Fatalf("cross-workspace channel error = %v, want invalid", err)
	}
	if _, err := service.CreateMessage(context.Background(), "alice", "workspace-1", thread.ID, "user", "hello channel"); err != nil {
		t.Fatal(err)
	}
}

type recordingChatStore struct {
	threads  []Thread
	messages []Message
}

func (r *recordingChatStore) CreateThread(_ context.Context, item Thread) error {
	r.threads = append(r.threads, item)
	return nil
}
func (r *recordingChatStore) ListThreads(context.Context, string, string) ([]Thread, error) {
	return append([]Thread(nil), r.threads...), nil
}
func (r *recordingChatStore) CreateMessage(_ context.Context, item Message) error {
	r.messages = append(r.messages, item)
	return nil
}
func (r *recordingChatStore) ListMessages(context.Context, string, string, string) ([]Message, error) {
	return append([]Message(nil), r.messages...), nil
}

func TestStoreWriteThrough(t *testing.T) {
	store := &recordingChatStore{}
	service := NewMemoryService(func() time.Time { return time.Unix(1, 0) }, allowAll{})
	service.SetStore(store)
	thread, err := service.CreateThread(context.Background(), "alice", "workspace-1", "Durable thread")
	if err != nil || len(store.threads) != 1 {
		t.Fatalf("thread=%+v stored=%+v err=%v", thread, store.threads, err)
	}
	if _, err := service.CreateMessage(context.Background(), "alice", "workspace-1", thread.ID, "user", "persist me"); err != nil {
		t.Fatal(err)
	}
	if len(store.messages) != 1 {
		t.Fatalf("expected message write-through, got %d", len(store.messages))
	}
}
