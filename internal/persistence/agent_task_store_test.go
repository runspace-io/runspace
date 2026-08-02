package persistence

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/runspace/runspace/internal/agentregistry"
	"github.com/runspace/runspace/internal/collaboration"
	"github.com/runspace/runspace/internal/workspace"
)

// messageID scopes a message identifier to its test. The column is a global
// primary key, so a shared literal lets one test's leftover row silently
// suppress another's insert through ON CONFLICT DO NOTHING.
func messageID(t *testing.T, name string) string {
	t.Helper()
	return t.Name() + "-" + name
}

// taskFixture creates the workspace, thread, and task that transcripts and
// questions hang off, and returns the task ID scoped to this test.
func taskFixture(t *testing.T) (*Store, context.Context, string) {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL is not set")
	}
	db, err := sql.Open("pgx", url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store := New(db)
	ctx := context.Background()
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	suffix := t.Name()
	workspaceID := "ws_" + suffix
	threadID := "thread_" + suffix
	taskID := "local_session_" + suffix
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM workspaces WHERE id=$1`, workspaceID)
	})
	if err := store.CreateWorkspaceWithMember(ctx, workspace.Workspace{
		ID: workspaceID, Slug: "slug-" + suffix, Name: suffix,
		CreatedBy: "admin", CreatedAt: now, UpdatedAt: now,
	}, workspace.Member{
		WorkspaceID: workspaceID, UserID: "admin", Role: workspace.RoleOwner, CreatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateThread(ctx, collaboration.Thread{
		ID: threadID, WorkspaceID: workspaceID, Title: "chat", CreatedBy: "admin", CreatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertAgentTask(ctx, agentregistry.AgentTask{
		ID: taskID, WorkspaceID: workspaceID, ThreadID: threadID, OwnerID: "admin",
		AgentID: "local_agent_abc", ResourceID: "resource-1", Title: "Investigate",
		Status: "running", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	return store, ctx, taskID
}

func TestAgentTaskMessagesRoundTripInOrder(t *testing.T) {
	store, ctx, taskID := taskFixture(t)
	base := time.Now().UTC().Truncate(time.Microsecond)
	if err := store.AppendAgentTaskMessages(ctx, taskID, []agentregistry.TaskMessage{
		{ID: messageID(t, "m2"), Role: "agent", Body: "Reading main.go", CreatedAt: base.Add(time.Second)},
		{ID: messageID(t, "m1"), Role: "user", Body: "investigate", CreatedAt: base},
	}); err != nil {
		t.Fatal(err)
	}
	stored, err := store.ListAgentTaskMessages(ctx, taskID)
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) != 2 || stored[0].ID != messageID(t, "m1") || stored[1].ID != messageID(t, "m2") {
		t.Fatalf("transcript out of order: %+v", stored)
	}
	if stored[0].Body != "investigate" || stored[1].Role != "agent" {
		t.Fatalf("transcript lost detail: %+v", stored)
	}
}

// A Host Agent that retries a push after a network error must not double-post.
func TestAgentTaskMessageAppendIsIdempotent(t *testing.T) {
	store, ctx, taskID := taskFixture(t)
	message := agentregistry.TaskMessage{
		ID: messageID(t, "m1"), Role: "agent", Body: "Reading main.go",
		CreatedAt: time.Now().UTC().Truncate(time.Microsecond),
	}
	for range 3 {
		if err := store.AppendAgentTaskMessages(
			ctx, taskID, []agentregistry.TaskMessage{message},
		); err != nil {
			t.Fatal(err)
		}
	}
	stored, err := store.ListAgentTaskMessages(ctx, taskID)
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) != 1 {
		t.Fatalf("retry duplicated the transcript: %+v", stored)
	}
}

func TestAgentTaskQuestionLifecycle(t *testing.T) {
	store, ctx, taskID := taskFixture(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	question := agentregistry.TaskQuestion{
		ID: "q_1_7", TaskID: taskID, Title: "Run rm -rf build/",
		Options: []agentregistry.QuestionOption{
			{ID: "once", Name: "Allow once", Kind: "allow_once"},
			{ID: "reject", Name: "Reject", Kind: "reject_once"},
		},
		Status: agentregistry.QuestionOpen, AskedAt: now, UpdatedAt: now,
	}
	if err := store.UpsertAgentTaskQuestion(ctx, question); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.GetAgentTaskQuestion(ctx, taskID, "q_1_7")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Title != question.Title || len(loaded.Options) != 2 ||
		loaded.Options[0].Name != "Allow once" || loaded.Status != agentregistry.QuestionOpen {
		t.Fatalf("question did not round-trip: %+v", loaded)
	}
	loaded.Status = agentregistry.QuestionAnswered
	loaded.AnsweredBy, loaded.AnsweredOption = "nahid", "once"
	loaded.UpdatedAt = now.Add(time.Second)
	if err := store.UpsertAgentTaskQuestion(ctx, loaded); err != nil {
		t.Fatal(err)
	}
	answered, err := store.GetAgentTaskQuestion(ctx, taskID, "q_1_7")
	if err != nil {
		t.Fatal(err)
	}
	if answered.Status != agentregistry.QuestionAnswered || answered.AnsweredBy != "nahid" ||
		answered.AnsweredOption != "once" {
		t.Fatalf("answer was not recorded: %+v", answered)
	}
	// Options must survive an answer, or a reloaded UI cannot label the choice.
	if len(answered.Options) != 2 {
		t.Fatalf("options lost on answer: %+v", answered)
	}
}

func TestMissingQuestionIsReportedAsUnavailable(t *testing.T) {
	store, ctx, taskID := taskFixture(t)
	_, err := store.GetAgentTaskQuestion(ctx, taskID, "q_absent")
	if !errors.Is(err, agentregistry.ErrTaskUnavailable) {
		t.Fatalf("error=%v", err)
	}
}

// Cancelling must close only what is still waiting; an answered question keeps
// its record of who decided.
func TestCancelOpenQuestionsLeavesAnsweredOnesAlone(t *testing.T) {
	store, ctx, taskID := taskFixture(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	base := agentregistry.TaskQuestion{
		TaskID: taskID, Title: "Run a command",
		Options: []agentregistry.QuestionOption{{ID: "once", Name: "Allow", Kind: "allow_once"}},
		AskedAt: now, UpdatedAt: now,
	}
	open := base
	open.ID, open.Status = "q_open", agentregistry.QuestionOpen
	answered := base
	answered.ID, answered.Status = "q_answered", agentregistry.QuestionAnswered
	answered.AnsweredBy, answered.AnsweredOption = "admin", "once"
	for _, question := range []agentregistry.TaskQuestion{open, answered} {
		if err := store.UpsertAgentTaskQuestion(ctx, question); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.CancelOpenAgentTaskQuestions(ctx, taskID, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	questions, err := store.ListAgentTaskQuestions(ctx, taskID)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]agentregistry.TaskQuestion{}
	for _, question := range questions {
		byID[question.ID] = question
	}
	if byID["q_open"].Status != agentregistry.QuestionCancelled {
		t.Fatalf("open question was not cancelled: %+v", byID["q_open"])
	}
	if byID["q_answered"].Status != agentregistry.QuestionAnswered ||
		byID["q_answered"].AnsweredBy != "admin" {
		t.Fatalf("answered question was overwritten: %+v", byID["q_answered"])
	}
}

// Deleting a task must not strand its transcript or questions.
func TestTaskDeletionCascadesToTranscriptAndQuestions(t *testing.T) {
	store, ctx, taskID := taskFixture(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	if err := store.AppendAgentTaskMessages(ctx, taskID, []agentregistry.TaskMessage{
		{ID: messageID(t, "m1"), Role: "user", Body: "investigate", CreatedAt: now},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertAgentTaskQuestion(ctx, agentregistry.TaskQuestion{
		ID: "q_1", TaskID: taskID, Title: "Run it",
		Options: []agentregistry.QuestionOption{{ID: "once", Name: "Allow", Kind: "allow_once"}},
		Status:  agentregistry.QuestionOpen, AskedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(
		ctx, `DELETE FROM agent_tasks WHERE id=$1`, taskID,
	); err != nil {
		t.Fatal(err)
	}
	messages, err := store.ListAgentTaskMessages(ctx, taskID)
	if err != nil {
		t.Fatal(err)
	}
	questions, err := store.ListAgentTaskQuestions(ctx, taskID)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 0 || len(questions) != 0 {
		t.Fatalf("orphans left behind: %d messages, %d questions", len(messages), len(questions))
	}
}
