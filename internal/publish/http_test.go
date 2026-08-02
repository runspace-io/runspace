package publish

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/runspace/runspace/internal/auth"
	"github.com/runspace/runspace/internal/contracts"
	"github.com/runspace/runspace/internal/workspace"
)

type publishAuth struct{}

func (publishAuth) CanWrite(context.Context, string, string) error { return nil }

type publishCatalog struct{ items []workspace.Repository }

func (c publishCatalog) ListRepositories(context.Context, string, string) ([]workspace.Repository, error) {
	return c.items, nil
}

type publishResolver struct{ path string }

func (r publishResolver) Root(context.Context, string, string) (string, error) { return r.path, nil }

type publishGit struct{ path string }

func (g *publishGit) Status(context.Context, string) (string, error) { return "changed", nil }
func (g *publishGit) CreateBranch(_ context.Context, request contracts.BranchRequest) (contracts.BranchResult, error) {
	g.path = request.Repository
	return contracts.BranchResult{Name: request.Name}, nil
}
func (g *publishGit) Commit(context.Context, string, string) (string, error) { return "sha", nil }
func (g *publishGit) Push(context.Context, string, string, string) error     { return nil }

type publishRemote struct{}

func (publishRemote) OpenPR(context.Context, contracts.PullRequest) (contracts.PullRequestResult, error) {
	return contracts.PullRequestResult{URL: "https://example/pr"}, nil
}

func TestPublishHandlerResolvesConnectedRepository(t *testing.T) {
	git := &publishGit{}
	service := New(git, publishRemote{})
	catalog := publishCatalog{items: []workspace.Repository{{ID: "repo-1", FullName: "acme/app", DefaultBranch: "main", CreatedAt: time.Now()}}}
	handler := NewHandler(service, publishAuth{}, catalog, publishResolver{path: "/checkout/repo-1"})
	request := httptest.NewRequest(http.MethodPost, "/workspaces/ws/runs/run/publish", strings.NewReader(`{"repository_id":"repo-1","repository_path":"/evil","branch":"forge/x","base":"main","commit_message":"x","title":"T"}`))
	request = request.WithContext(auth.WithUserID(request.Context(), "alice"))
	response := httptest.NewRecorder()
	router := chi.NewRouter()
	handler.RegisterRoutes(router)
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || git.path != "" {
		t.Fatalf("status=%d path=%q", response.Code, git.path)
	}
	request = httptest.NewRequest(http.MethodPost, "/workspaces/ws/runs/run-2/publish", strings.NewReader(`{"repository_id":"repo-1","branch":"forge/x","base":"main","commit_message":"x","title":"T"}`))
	request = request.WithContext(auth.WithUserID(request.Context(), "alice"))
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusAccepted || git.path != "/checkout/repo-1" {
		t.Fatalf("status=%d path=%q", response.Code, git.path)
	}
}

func TestPublishHandlerRejectsDisconnectedRepository(t *testing.T) {
	handler := NewHandler(New(&publishGit{}, publishRemote{}), publishAuth{}, publishCatalog{}, publishResolver{path: "/checkout"})
	request := httptest.NewRequest(http.MethodPost, "/workspaces/ws/runs/run/publish", strings.NewReader(`{"repository_id":"missing","branch":"forge/x","base":"main","commit_message":"x","title":"T"}`))
	request = request.WithContext(auth.WithUserID(request.Context(), "alice"))
	response := httptest.NewRecorder()
	router := chi.NewRouter()
	handler.RegisterRoutes(router)
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", response.Code)
	}
}
