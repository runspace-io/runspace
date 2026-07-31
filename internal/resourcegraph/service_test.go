package resourcegraph

import (
	"context"
	"errors"
	"testing"
	"time"
)

type testAuthorizer struct{ denied bool }

func (a testAuthorizer) CanRead(context.Context, string, string) error {
	if a.denied {
		return errors.New("denied")
	}
	return nil
}
func (a testAuthorizer) CanWrite(context.Context, string, string) error {
	if a.denied {
		return errors.New("denied")
	}
	return nil
}

func TestProjectsAgentWorkIntoThreadScopedGraph(t *testing.T) {
	now := time.Date(2026, 7, 30, 10, 0, 0, 0, time.UTC)
	service := New(testAuthorizer{}, func() time.Time { return now })
	ctx := context.Background()
	if err := service.ProjectDiscussion(
		ctx, "thread_1", "ws_1", "channel_1", "nahid", "Fix terminal", now,
	); err != nil {
		t.Fatal(err)
	}
	if err := service.ProjectResource(
		ctx, "repo_1", "ws_1", "nahid", "folder", "runspace", now,
	); err != nil {
		t.Fatal(err)
	}
	if err := service.ProjectAgentTask(
		ctx, "local_session_1", "ws_1", "thread_1", "nahid", "agent_1",
		"repo_1", "Restore terminal input", "running", now, now,
	); err != nil {
		t.Fatal(err)
	}
	items, err := service.ListNodes(ctx, "nahid", "ws_1", Query{
		Kind: KindTask, ThreadID: "thread_1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Title != "Restore terminal input" {
		t.Fatalf("unexpected tasks: %#v", items)
	}
	graphContext, err := service.GetContext(ctx, "nahid", "ws_1", items[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(graphContext.Outgoing) != 2 {
		t.Fatalf("expected discussion and resource edges, got %#v", graphContext.Outgoing)
	}
}

func TestRejectsUnknownKindsAndUnauthorizedReads(t *testing.T) {
	service := New(testAuthorizer{}, time.Now)
	_, err := service.UpsertNode(context.Background(), "nahid", Node{
		ID: "unknown:1", WorkspaceID: "ws_1", Kind: "unknown", Type: "thing", Title: "Thing",
	})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected invalid kind, got %v", err)
	}
	denied := New(testAuthorizer{denied: true}, time.Now)
	_, err = denied.ListNodes(context.Background(), "nahid", "ws_1", Query{})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}
