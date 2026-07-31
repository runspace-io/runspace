package repository

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/runspace/runspace/internal/contracts"
	"github.com/runspace/runspace/internal/sandbox"
	"github.com/runspace/runspace/internal/workspace"
)

type fakeCloner struct{ request contracts.CloneRequest }

func (f *fakeCloner) Clone(_ context.Context, request contracts.CloneRequest) (contracts.CloneResult, error) {
	f.request = request
	if err := os.MkdirAll(request.Destination, 0o750); err != nil {
		return contracts.CloneResult{}, err
	}
	return contracts.CloneResult{Path: request.Destination, Ref: request.Ref}, nil
}

func TestCloneUsesRepositoryMetadataAndScopedDestination(t *testing.T) {
	workspaces := workspace.NewMemoryService(func() time.Time { return time.Unix(1, 0) })
	created, err := workspaces.CreateWorkspace(context.Background(), "user", workspace.CreateWorkspaceRequest{Name: "Forge"})
	if err != nil {
		t.Fatal(err)
	}
	repo, err := workspaces.ConnectRepository(context.Background(), "user", created.ID, workspace.ConnectRepositoryRequest{Provider: "github", FullName: "org/repo", CloneURL: "https://github.com/org/repo.git", DefaultBranch: "main"})
	if err != nil {
		t.Fatal(err)
	}
	resolver, err := sandbox.NewLayoutResolver(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cloner := &fakeCloner{}
	service, err := NewService(workspaces, resolver, cloner)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Clone(context.Background(), "user", created.ID, repo.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Ref != "main" || cloner.request.RepositoryURL != repo.CloneURL {
		t.Fatalf("result=%+v request=%+v", result, cloner.request)
	}
}

func TestPrepareExistingCheckoutRepairsUnseededRemotePlaceholder(t *testing.T) {
	destination := t.TempDir()
	gitDirectory := filepath.Join(destination, ".git")
	if err := os.Mkdir(gitDirectory, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(gitDirectory, "config"),
		[]byte("[core]\n\trepositoryformatversion = 0\n"),
		0o640,
	); err != nil {
		t.Fatal(err)
	}
	exists, err := prepareExistingCheckout(destination, workspace.Repository{
		Provider: "mirror",
		CloneURL: "https://github.com/runspace/demo.git",
	})
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("unseeded placeholder was treated as a complete checkout")
	}
	if _, err := os.Stat(gitDirectory); !os.IsNotExist(err) {
		t.Fatalf("placeholder metadata was not removed: %v", err)
	}
}
