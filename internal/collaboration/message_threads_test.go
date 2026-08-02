package collaboration

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCreateMessageThreadIsIdempotentPerVisibility(t *testing.T) {
	service := NewMemoryService(time.Now, allowAll{})
	channel, err := service.CreateChannel(context.Background(), "alice", "workspace-1", "general", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	parent, err := service.CreateThread(context.Background(), "alice", "workspace-1", "general chat", channel.ID)
	if err != nil {
		t.Fatal(err)
	}
	root, err := service.CreateMessage(context.Background(), "alice", "workspace-1", parent.ID, "user", "the build is failing")
	if err != nil {
		t.Fatal(err)
	}

	first, err := service.CreateMessageThread(context.Background(), "bob", "workspace-1", parent.ID, root.ID, "public")
	if err != nil {
		t.Fatal(err)
	}
	if first.Visibility != ThreadVisibilityPublic || first.ParentMessageID != root.ID || first.ChannelID != channel.ID {
		t.Fatalf("unexpected thread: %+v", first)
	}
	again, err := service.CreateMessageThread(context.Background(), "carol", "workspace-1", parent.ID, root.ID, "public")
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != first.ID {
		t.Fatalf("expected the same public thread to be reused, got %q and %q", first.ID, again.ID)
	}

	bobPrivate, err := service.CreateMessageThread(context.Background(), "bob", "workspace-1", parent.ID, root.ID, "private")
	if err != nil {
		t.Fatal(err)
	}
	carolPrivate, err := service.CreateMessageThread(context.Background(), "carol", "workspace-1", parent.ID, root.ID, "private")
	if err != nil {
		t.Fatal(err)
	}
	if bobPrivate.ID == carolPrivate.ID {
		t.Fatalf("expected separate private threads per creator, got the same one: %q", bobPrivate.ID)
	}
	bobAgain, err := service.CreateMessageThread(context.Background(), "bob", "workspace-1", parent.ID, root.ID, "private")
	if err != nil {
		t.Fatal(err)
	}
	if bobAgain.ID != bobPrivate.ID {
		t.Fatalf("expected bob's private thread to be reused, got %q and %q", bobPrivate.ID, bobAgain.ID)
	}
}

func TestListMessageThreadsHidesOthersPrivateThreads(t *testing.T) {
	service := NewMemoryService(time.Now, allowAll{})
	parent, err := service.CreateThread(context.Background(), "alice", "workspace-1", "general chat")
	if err != nil {
		t.Fatal(err)
	}
	root, err := service.CreateMessage(context.Background(), "alice", "workspace-1", parent.ID, "user", "hello")
	if err != nil {
		t.Fatal(err)
	}
	publicThread, err := service.CreateMessageThread(context.Background(), "alice", "workspace-1", parent.ID, root.ID, "public")
	if err != nil {
		t.Fatal(err)
	}
	bobPrivate, err := service.CreateMessageThread(context.Background(), "bob", "workspace-1", parent.ID, root.ID, "private")
	if err != nil {
		t.Fatal(err)
	}

	fromBob, err := service.ListMessageThreads(context.Background(), "bob", "workspace-1", parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(fromBob) != 2 {
		t.Fatalf("bob should see the public thread and his own private thread, got %d: %+v", len(fromBob), fromBob)
	}

	fromCarol, err := service.ListMessageThreads(context.Background(), "carol", "workspace-1", parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(fromCarol) != 1 || fromCarol[0].ID != publicThread.ID {
		t.Fatalf("carol should see only the public thread, got %+v", fromCarol)
	}
	for _, thread := range fromCarol {
		if thread.ID == bobPrivate.ID {
			t.Fatalf("carol must never see bob's private thread")
		}
	}
}

func TestListPrivateThreadsScopedToCreator(t *testing.T) {
	service := NewMemoryService(time.Now, allowAll{})
	parent, err := service.CreateThread(context.Background(), "alice", "workspace-1", "general chat")
	if err != nil {
		t.Fatal(err)
	}
	root, err := service.CreateMessage(context.Background(), "alice", "workspace-1", parent.ID, "user", "hello")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateMessageThread(context.Background(), "bob", "workspace-1", parent.ID, root.ID, "private"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateMessageThread(context.Background(), "carol", "workspace-1", parent.ID, root.ID, "private"); err != nil {
		t.Fatal(err)
	}

	bobPrivate, err := service.ListPrivateThreads(context.Background(), "bob", "workspace-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(bobPrivate) != 1 || bobPrivate[0].CreatedBy != "bob" {
		t.Fatalf("expected exactly bob's own private thread, got %+v", bobPrivate)
	}
}

func TestPrivateThreadMessagesHiddenFromNonCreator(t *testing.T) {
	service := NewMemoryService(time.Now, allowAll{})
	parent, err := service.CreateThread(context.Background(), "alice", "workspace-1", "general chat")
	if err != nil {
		t.Fatal(err)
	}
	root, err := service.CreateMessage(context.Background(), "alice", "workspace-1", parent.ID, "user", "hello")
	if err != nil {
		t.Fatal(err)
	}
	private, err := service.CreateMessageThread(context.Background(), "bob", "workspace-1", parent.ID, root.ID, "private")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateMessage(context.Background(), "bob", "workspace-1", private.ID, "user", "just between me and the agent"); err != nil {
		t.Fatal(err)
	}

	if _, err := service.CreateMessage(context.Background(), "carol", "workspace-1", private.ID, "user", "can I see this?"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound writing into someone else's private thread, got %v", err)
	}
	if _, err := service.ListMessages(context.Background(), "carol", "workspace-1", private.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound reading someone else's private thread, got %v", err)
	}

	ownMessages, err := service.ListMessages(context.Background(), "bob", "workspace-1", private.ID)
	if err != nil || len(ownMessages) != 1 {
		t.Fatalf("expected bob to read his own private thread, messages=%v err=%v", ownMessages, err)
	}
}

func TestListThreadsExcludesMessageSubthreads(t *testing.T) {
	service := NewMemoryService(time.Now, allowAll{})
	parent, err := service.CreateThread(context.Background(), "alice", "workspace-1", "general chat")
	if err != nil {
		t.Fatal(err)
	}
	root, err := service.CreateMessage(context.Background(), "alice", "workspace-1", parent.ID, "user", "hello")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateMessageThread(context.Background(), "alice", "workspace-1", parent.ID, root.ID, "public"); err != nil {
		t.Fatal(err)
	}

	threads, err := service.ListThreads(context.Background(), "alice", "workspace-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(threads) != 1 || threads[0].ID != parent.ID {
		t.Fatalf("expected ListThreads to return only the channel thread, got %+v", threads)
	}
}
