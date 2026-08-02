package persistence

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/runspace/runspace/internal/collaboration"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// TestPostgresMessageThreads exercises the real SQL for subthreads end to
// end — the unit tests in internal/collaboration only cover MemoryService's
// in-memory paths, and the new queries here (updated CreateThread/ListThreads,
// the two new Store methods, and ListMessages' visibility filter) are
// otherwise unverified against a real database.
func TestPostgresMessageThreads(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL is not set")
	}
	db, err := sql.Open("pgx", url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	store := New(db)
	ctx := context.Background()
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	workspaceID := "test_ws_" + time.Now().UTC().Format("20060102150405.000000")
	if _, err := db.ExecContext(ctx, `INSERT INTO workspaces (id,slug,name,created_by,created_at,updated_at) VALUES ($1,$1,'message threads test','alice',now(),now())`, workspaceID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, `DELETE FROM workspaces WHERE id=$1`, workspaceID) })
	for _, member := range []string{"alice", "bob"} {
		if _, err := db.ExecContext(ctx, `INSERT INTO workspace_members (workspace_id,user_id,role,created_at) VALUES ($1,$2,'member',now())`, workspaceID, member); err != nil {
			t.Fatal(err)
		}
	}

	now := time.Now().UTC()
	parent := collaboration.Thread{ID: "test_thread_parent_" + workspaceID, WorkspaceID: workspaceID, Title: "general chat", CreatedBy: "alice", CreatedAt: now}
	if err := store.CreateThread(ctx, parent); err != nil {
		t.Fatal(err)
	}
	root := collaboration.Message{ID: "test_msg_root_" + workspaceID, ThreadID: parent.ID, ActorID: "alice", ActorType: "user", Body: "the build is failing", CreatedAt: now}
	if err := store.CreateMessage(ctx, root); err != nil {
		t.Fatal(err)
	}

	public := collaboration.Thread{
		ID: "test_thread_public_" + workspaceID, WorkspaceID: workspaceID,
		ParentThreadID: parent.ID, ParentMessageID: root.ID,
		Visibility: collaboration.ThreadVisibilityPublic, CreatedBy: "alice", CreatedAt: now,
	}
	if err := store.CreateThread(ctx, public); err != nil {
		t.Fatal(err)
	}
	private := collaboration.Thread{
		ID: "test_thread_private_" + workspaceID, WorkspaceID: workspaceID,
		ParentThreadID: parent.ID, ParentMessageID: root.ID,
		Visibility: collaboration.ThreadVisibilityPrivate, CreatedBy: "bob", CreatedAt: now,
	}
	if err := store.CreateThread(ctx, private); err != nil {
		t.Fatal(err)
	}
	privateMessage := collaboration.Message{ID: "test_msg_private_" + workspaceID, ThreadID: private.ID, ActorID: "bob", ActorType: "user", Body: "just between me and the agent", CreatedAt: now}
	if err := store.CreateMessage(ctx, privateMessage); err != nil {
		t.Fatal(err)
	}

	t.Run("ListThreads excludes subthreads", func(t *testing.T) {
		threads, err := store.ListThreads(ctx, "alice", workspaceID)
		if err != nil {
			t.Fatal(err)
		}
		if len(threads) != 1 || threads[0].ID != parent.ID {
			t.Fatalf("expected only the parent thread, got %+v", threads)
		}
	})

	t.Run("ListThreadsByParentThreadID returns both subthreads unfiltered", func(t *testing.T) {
		threads, err := store.ListThreadsByParentThreadID(ctx, workspaceID, parent.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(threads) != 2 {
			t.Fatalf("expected both subthreads (filtering is the service layer's job), got %+v", threads)
		}
	})

	t.Run("ListThreadsByCreator scopes to visibility and creator", func(t *testing.T) {
		bobPrivate, err := store.ListThreadsByCreator(ctx, workspaceID, "bob", collaboration.ThreadVisibilityPrivate)
		if err != nil {
			t.Fatal(err)
		}
		if len(bobPrivate) != 1 || bobPrivate[0].ID != private.ID {
			t.Fatalf("expected bob's private thread, got %+v", bobPrivate)
		}
		alicePrivate, err := store.ListThreadsByCreator(ctx, workspaceID, "alice", collaboration.ThreadVisibilityPrivate)
		if err != nil {
			t.Fatal(err)
		}
		if len(alicePrivate) != 0 {
			t.Fatalf("alice created no private threads, got %+v", alicePrivate)
		}
	})

	t.Run("ListMessages hides a private thread from a non-creator", func(t *testing.T) {
		asBob, err := store.ListMessages(ctx, "bob", workspaceID, private.ID)
		if err != nil || len(asBob) != 1 {
			t.Fatalf("bob should read his own private thread, messages=%v err=%v", asBob, err)
		}
		asAlice, err := store.ListMessages(ctx, "alice", workspaceID, private.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(asAlice) != 0 {
			t.Fatalf("alice must not see bob's private thread messages, got %+v", asAlice)
		}
	})

	t.Run("ListMessages still returns a public subthread to any member", func(t *testing.T) {
		publicMessage := collaboration.Message{ID: "test_msg_public_" + workspaceID, ThreadID: public.ID, ActorID: "alice", ActorType: "user", Body: "let's dig into this", CreatedAt: now}
		if err := store.CreateMessage(ctx, publicMessage); err != nil {
			t.Fatal(err)
		}
		asBob, err := store.ListMessages(ctx, "bob", workspaceID, public.ID)
		if err != nil || len(asBob) != 1 {
			t.Fatalf("bob should read the public subthread, messages=%v err=%v", asBob, err)
		}
	})
}
