package events

import (
	"context"
	"testing"
	"time"

	"github.com/runspace/runspace/internal/contracts"
)

func TestEventSubjectIsVersionedAndRoutable(t *testing.T) {
	event := contracts.EventEnvelope{Type: "message.created", Version: 1}
	if got, want := eventSubject(event), "evt.message.created.v1"; got != want {
		t.Fatalf("subject=%q want=%q", got, want)
	}
}

func TestEventSubjectDoesNotDuplicateVersionSuffix(t *testing.T) {
	event := contracts.EventEnvelope{Type: contracts.EventGitStatusChanged, Version: 1}
	if got, want := eventSubject(event), "evt.git.status.changed.v1"; got != want {
		t.Fatalf("subject=%q want=%q", got, want)
	}
}

func TestNATSPublisherRejectsInvalidEventsWithoutConnection(t *testing.T) {
	publisher := &NATSPublisher{}
	event := contracts.EventEnvelope{OccurredAt: time.Now()}
	if err := publisher.Publish(context.Background(), event); err == nil {
		t.Fatal("expected invalid event error")
	}
}
