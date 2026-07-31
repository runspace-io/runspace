package runs

import (
	"context"
	"testing"
	"time"

	"github.com/runspace/runspace/internal/collaboration"
	"github.com/runspace/runspace/internal/contracts"
	"github.com/runspace/runspace/internal/workspace"
)

type runRootResolver struct {
	path string
}

func (r runRootResolver) Root(context.Context, string, string) (string, error) {
	return r.path, nil
}

func runTestClock() time.Time {
	return time.Date(2026, time.July, 29, 0, 0, 0, 0, time.UTC)
}

func TestAssignWorkingDirectoryRequiresConnectedRepository(t *testing.T) {
	ctx := context.Background()
	workspaces := workspace.NewMemoryService(runTestClock)
	model, err := workspaces.CreateWorkspace(ctx, "admin", workspace.CreateWorkspaceRequest{Name: "Runspace"})
	if err != nil {
		t.Fatal(err)
	}
	repository, err := workspaces.ConnectRepository(ctx, "admin", model.ID, workspace.ConnectRepositoryRequest{
		Provider: "local", FullName: "runspace/local", CloneURL: "file:///repo", DefaultBranch: "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(New(nil, nil), workspaces, nil, runRootResolver{path: "/workspace/repo"})
	spawn := contracts.SpawnRequest{WorkspaceID: model.ID, Repository: repository.ID}
	if err := handler.assignWorkingDirectory(ctx, "admin", &spawn); err != nil {
		t.Fatal(err)
	}
	if spawn.WorkingDirectory != "/workspace/repo" {
		t.Fatalf("working directory=%q", spawn.WorkingDirectory)
	}
	spawn.Repository = "not-connected"
	if err := handler.assignWorkingDirectory(ctx, "admin", &spawn); err == nil {
		t.Fatal("expected disconnected repository error")
	}
}

func TestResolveSpawnContextUsesChannelAgent(t *testing.T) {
	ctx := context.Background()
	workspaces := workspace.NewMemoryService(runTestClock)
	model, err := workspaces.CreateWorkspace(ctx, "admin", workspace.CreateWorkspaceRequest{Name: "Runspace"})
	if err != nil {
		t.Fatal(err)
	}
	chat := collaboration.NewMemoryService(runTestClock, workspaces)
	channel, err := chat.CreateChannel(
		ctx, "admin", model.ID, "engineering", "", "",
		map[string]any{"agent": map[string]any{"command": "codex-acp"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	thread, err := chat.CreateThread(ctx, "admin", model.ID, "Build", channel.ID)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(New(nil, nil), workspaces, chat, nil)
	spawn := contracts.SpawnRequest{WorkspaceID: model.ID, ThreadID: thread.ID}
	if err := handler.resolveSpawnContext(ctx, "admin", &spawn); err != nil {
		t.Fatal(err)
	}
	if spawn.ChannelID != channel.ID || spawn.AgentCommand != "codex-acp" {
		t.Fatalf("spawn context=%+v", spawn)
	}
}
