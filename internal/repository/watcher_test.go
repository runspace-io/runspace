package repository

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/runspace/runspace/internal/contracts"
)

type watcherStatus struct {
	mu     sync.Mutex
	value  string
	called chan struct{}
}

func (s *watcherStatus) Status(context.Context, string) (string, error) {
	s.mu.Lock()
	value := s.value
	s.mu.Unlock()
	select {
	case s.called <- struct{}{}:
	default:
	}
	return value, nil
}

type watcherPublisher struct {
	mu     sync.Mutex
	events []contracts.EventEnvelope
	called chan struct{}
}

func (p *watcherPublisher) Publish(_ context.Context, event contracts.EventEnvelope) error {
	p.mu.Lock()
	p.events = append(p.events, event)
	p.mu.Unlock()
	select {
	case p.called <- struct{}{}:
	default:
	}
	return nil
}

func TestWatcherDebouncesAndPublishesAuthoritativeStatus(t *testing.T) {
	root := t.TempDir()
	status := &watcherStatus{value: " M app.go", called: make(chan struct{}, 4)}
	publisher := &watcherPublisher{called: make(chan struct{}, 4)}
	watcher, err := NewWatcherWithDebounce(root, "workspace-1", "repo-1", 25*time.Millisecond, status, publisher)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := watcher.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer watcher.Stop()
	writeChanges(t, root)
	waitForEvent(t, publisher)
	time.Sleep(60 * time.Millisecond)
	publisher.mu.Lock()
	defer publisher.mu.Unlock()
	assertStatusEvent(t, publisher.events)
}

func writeChanges(t *testing.T, root string) {
	t.Helper()
	for index := 0; index < 3; index++ {
		if err := os.WriteFile(filepath.Join(root, "app.go"), []byte(string(rune('a'+index))), 0o640); err != nil {
			t.Fatal(err)
		}
	}
}

func waitForEvent(t *testing.T, publisher *watcherPublisher) {
	t.Helper()
	select {
	case <-publisher.called:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for status event")
	}
}

func assertStatusEvent(t *testing.T, events []contracts.EventEnvelope) {
	t.Helper()
	if len(events) != 1 {
		t.Fatalf("expected one debounced event, got %d", len(events))
	}
	var payload StatusChanged
	if err := json.Unmarshal(events[0].Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Status != "M app.go" || events[0].RepositoryID != "repo-1" {
		t.Fatalf("unexpected status event: %#v", payload)
	}
}

func TestWatcherTracksCreatedDirectories(t *testing.T) {
	root := t.TempDir()
	status := &watcherStatus{value: "?? nested/new.txt", called: make(chan struct{}, 4)}
	publisher := &watcherPublisher{called: make(chan struct{}, 4)}
	watcher, err := NewWatcherWithDebounce(root, "workspace-1", "repo-1", 10*time.Millisecond, status, publisher)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := watcher.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer watcher.Stop()
	nested := filepath.Join(root, "nested")
	if err := os.Mkdir(nested, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "new.txt"), []byte("change"), 0o640); err != nil {
		t.Fatal(err)
	}
	select {
	case <-publisher.called:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for nested status event")
	}
}

func TestWatcherLifecycleValidation(t *testing.T) {
	status := &watcherStatus{called: make(chan struct{}, 1)}
	publisher := &watcherPublisher{called: make(chan struct{}, 1)}
	if _, err := NewWatcherWithDebounce("", "workspace", "repo", time.Second, status, publisher); err == nil {
		t.Fatal("expected empty path validation")
	}
	if _, err := NewWatcherWithDebounce(t.TempDir(), "workspace", "repo", 0, status, publisher); err == nil {
		t.Fatal("expected debounce validation")
	}
	watcher, err := NewWatcherWithDebounce(t.TempDir(), "workspace", "repo", time.Second, status, publisher)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := watcher.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := watcher.Start(ctx); err == nil {
		t.Fatal("expected duplicate start validation")
	}
	watcher.Stop()
	watcher.Stop()
}
