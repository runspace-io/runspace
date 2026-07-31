package collaboration

import (
	"context"
	"testing"

	"github.com/runspace/runspace/internal/contracts"
)

type recordingPublisher struct{ events []contracts.EventEnvelope }

func (p *recordingPublisher) Publish(_ context.Context, event contracts.EventEnvelope) error {
	p.events = append(p.events, event)
	return nil
}

func TestChatMutationsPublishDomainEvents(t *testing.T) {
	publisher := &recordingPublisher{}
	service := NewMemoryService(nil, allowAll{})
	service.SetPublisher(publisher)
	thread, err := service.CreateThread(context.Background(), "alice", "workspace-1", "Ship it")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateMessage(context.Background(), "alice", "workspace-1", thread.ID, "user", "hello"); err != nil {
		t.Fatal(err)
	}
	if len(publisher.events) != 2 || publisher.events[0].Type != "thread.created" || publisher.events[1].Type != "message.created" {
		t.Fatalf("published events=%+v", publisher.events)
	}
}
