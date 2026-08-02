package agentregistry

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/runspace/runspace/internal/contracts"
)

type recordingPublisher struct{ events []contracts.EventEnvelope }

func (p *recordingPublisher) Publish(_ context.Context, event contracts.EventEnvelope) error {
	p.events = append(p.events, event)
	return nil
}

type memoryTaskMessageStore struct{ messages map[string][]TaskMessage }

func (s *memoryTaskMessageStore) AppendAgentTaskMessages(
	_ context.Context, taskID string, messages []TaskMessage,
) error {
	if s.messages == nil {
		s.messages = map[string][]TaskMessage{}
	}
	for _, message := range messages {
		if containsMessage(s.messages[taskID], message.ID) {
			continue
		}
		s.messages[taskID] = append(s.messages[taskID], message)
	}
	return nil
}

func (s *memoryTaskMessageStore) ListAgentTaskMessages(
	_ context.Context, taskID string,
) ([]TaskMessage, error) {
	return s.messages[taskID], nil
}

func containsMessage(messages []TaskMessage, id string) bool {
	for _, message := range messages {
		if message.ID == id {
			return true
		}
	}
	return false
}

func streamService(t *testing.T) (*Service, *recordingPublisher, *memoryTaskMessageStore) {
	t.Helper()
	service := New(func() time.Time { return time.Unix(10, 0) }, registryAuthorizer{})
	publisher := &recordingPublisher{}
	store := &memoryTaskMessageStore{}
	service.SetEventPublisher(publisher)
	service.SetTaskMessageStore(store)
	if _, err := service.Upsert(context.Background(), "admin", Installation{
		ID: "local_agent_abc", RegistryID: "codex-acp", Name: "Codex",
		Protocol: "acp", Placement: "host", Status: "ready",
	}); err != nil {
		t.Fatal(err)
	}
	return service, publisher, store
}

func sampleUpdate(body string, messageID string) TaskStreamUpdate {
	return TaskStreamUpdate{
		WorkspaceID: "ws_1", ThreadID: "thread_1", AgentID: "local_agent_abc",
		ResourceID: "resource_1", Title: "Investigate the failing test", Status: "running",
		Messages: []TaskMessage{{
			ID: messageID, Role: "agent", Body: body, CreatedAt: time.Unix(20, 0).UTC(),
		}},
	}
}

func TestRecordTaskStreamCreatesTaskAndStoresTranscript(t *testing.T) {
	service, _, store := streamService(t)
	err := service.RecordTaskStream(
		context.Background(), "admin", "local_session_1", sampleUpdate("Reading main.go", "m1"),
	)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := service.ListTaskMessages(context.Background(), "admin", "local_session_1")
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) != 1 || stored[0].Body != "Reading main.go" {
		t.Fatalf("transcript not stored: %+v", store.messages)
	}
}

// The realtime hub fans out per workspace, not per grant, so a published event
// must never carry the private transcript body.
func TestRecordTaskStreamPublishesWithoutMessageBodies(t *testing.T) {
	service, publisher, _ := streamService(t)
	secret := "the API key is hunter2"
	if err := service.RecordTaskStream(
		context.Background(), "admin", "local_session_1", sampleUpdate(secret, "m1"),
	); err != nil {
		t.Fatal(err)
	}
	if len(publisher.events) != 2 {
		t.Fatalf("expected message and status events, got %d", len(publisher.events))
	}
	for _, event := range publisher.events {
		payload, err := json.Marshal(event.Payload)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(payload), "hunter2") {
			t.Fatalf("event %s leaked the transcript body: %s", event.Type, payload)
		}
	}
	if publisher.events[0].Type != contracts.EventAgentTaskMessage ||
		publisher.events[1].Type != contracts.EventAgentTaskStatus {
		t.Fatalf("unexpected event types: %s %s",
			publisher.events[0].Type, publisher.events[1].Type)
	}
}

func TestRecordTaskStreamRejectsNonOwner(t *testing.T) {
	service, _, _ := streamService(t)
	err := service.RecordTaskStream(
		context.Background(), "nahid", "local_session_1", sampleUpdate("Reading main.go", "m1"),
	)
	if err == nil {
		t.Fatal("a non-owner pushed a transcript into another user's task")
	}
}

func TestListTaskMessagesRequiresAccess(t *testing.T) {
	service, _, _ := streamService(t)
	if err := service.RecordTaskStream(
		context.Background(), "admin", "local_session_1", sampleUpdate("Reading main.go", "m1"),
	); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ListTaskMessages(
		context.Background(), "nahid", "local_session_1",
	); err == nil {
		t.Fatal("transcript readable without a grant")
	}
	if _, err := service.GrantTaskAccess(context.Background(), "admin", TaskGrant{
		TaskID: "local_session_1", WorkspaceID: "ws_1", AgentID: "local_agent_abc",
		PrincipalID: "nahid", Role: "viewer",
	}); err != nil {
		t.Fatal(err)
	}
	granted, err := service.ListTaskMessages(context.Background(), "nahid", "local_session_1")
	if err != nil {
		t.Fatal(err)
	}
	if len(granted) != 1 {
		t.Fatalf("grantee could not read the transcript: %+v", granted)
	}
}

// A Host Agent that retries a push after a network error must not double-post.
func TestRecordTaskStreamIsIdempotentPerMessage(t *testing.T) {
	service, _, _ := streamService(t)
	update := sampleUpdate("Reading main.go", "m1")
	for range 2 {
		if err := service.RecordTaskStream(
			context.Background(), "admin", "local_session_1", update,
		); err != nil {
			t.Fatal(err)
		}
	}
	stored, err := service.ListTaskMessages(context.Background(), "admin", "local_session_1")
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) != 1 {
		t.Fatalf("retry duplicated the transcript: %+v", stored)
	}
}
