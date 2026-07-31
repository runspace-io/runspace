package realtime

import (
	"context"
	"testing"
	"time"

	"github.com/runspace/runspace/internal/contracts"
)

func TestMemoryHubRoutesByWorkspaceAndClosesOnce(t *testing.T) {
	hub := NewMemoryHub()
	channel, closeSubscription, err := hub.Subscribe(context.Background(), "workspace-1")
	if err != nil {
		t.Fatal(err)
	}
	event, err := contracts.NewEvent("event-1", "run.started", "workspace-1", "agent-1", "agent", map[string]string{"run_id": "run-1"}, time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if err := hub.Publish(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	select {
	case received := <-channel:
		if received.ID != event.ID {
			t.Fatalf("received %q, expected %q", received.ID, event.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
	closeSubscription()
	closeSubscription()
	if _, open := <-channel; open {
		t.Fatal("expected closed subscription")
	}
}

func TestMemoryHubReplaysAfterLastEventWithoutDuplicates(t *testing.T) {
	hub := NewMemoryHub()
	for _, id := range []string{"event-1", "event-2", "event-3"} {
		event, err := contracts.NewEvent(id, "message.created", "workspace-1", "user-1", "user", map[string]string{"id": id}, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if err := hub.Publish(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	channel, closeSubscription, err := hub.SubscribeSince(context.Background(), "workspace-1", "event-1")
	if err != nil {
		t.Fatal(err)
	}
	defer closeSubscription()
	for _, expected := range []string{"event-2", "event-3"} {
		select {
		case event := <-channel:
			if event.ID != expected {
				t.Fatalf("received %q, expected %q", event.ID, expected)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for replay")
		}
	}
}
