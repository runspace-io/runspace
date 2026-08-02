package sandbox

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/runspace/runspace/internal/auth"
	"github.com/runspace/runspace/internal/git"
	"github.com/runspace/runspace/internal/workspace"
)

func TestHandlerAuthorizesAndReads(t *testing.T) {
	service, _ := testService(t)
	authorizer := workspace.NewMemoryService(time.Now)
	ws, err := authorizer.CreateWorkspace(t.Context(), "alice", workspace.CreateWorkspaceRequest{Name: "Demo"})
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(service, authorizer, nil, nil)
	request := httptest.NewRequest(http.MethodGet, "/workspaces/"+ws.ID+"/repositories/repo-1/file?path=src/main.go", nil)
	request = request.WithContext(auth.WithUserID(request.Context(), "alice"))
	recorder := httptest.NewRecorder()
	handler.Routes().ServeHTTP(recorder, request)
	if recorder.Code != 200 {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestHandlerRejectsAnonymousAndTraversal(t *testing.T) {
	service, _ := testService(t)
	authorizer := workspace.NewMemoryService(time.Now)
	ws, err := authorizer.CreateWorkspace(t.Context(), "alice", workspace.CreateWorkspaceRequest{Name: "Demo"})
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(service, authorizer, nil, nil)
	anonymous := httptest.NewRequest(http.MethodGet, "/workspaces/"+ws.ID+"/repositories/repo-1/tree", nil)
	recorder := httptest.NewRecorder()
	handler.Routes().ServeHTTP(recorder, anonymous)
	if recorder.Code != 401 {
		t.Fatalf("anonymous status = %d", recorder.Code)
	}
	request := httptest.NewRequest(http.MethodGet, "/workspaces/"+ws.ID+"/repositories/repo-1/file?path=..%2Fsecret", nil)
	request = request.WithContext(auth.WithUserID(request.Context(), "alice"))
	recorder = httptest.NewRecorder()
	handler.Routes().ServeHTTP(recorder, request)
	if recorder.Code != 400 {
		t.Fatalf("traversal status = %d", recorder.Code)
	}
}

type reviewDependencies struct{}

func (reviewDependencies) CanRead(context.Context, string, string) error { return nil }
func (reviewDependencies) ListRepositories(
	context.Context,
	string,
	string,
) ([]workspace.Repository, error) {
	return []workspace.Repository{{ID: "repo-1"}}, nil
}
func (reviewDependencies) Diff(context.Context, string) (string, error) { return "", nil }
func (reviewDependencies) ChangedFiles(context.Context, string) ([]git.Change, error) {
	return []git.Change{{Path: "src/app.ts", Status: "modified"}}, nil
}
func (reviewDependencies) FileContents(
	context.Context,
	string,
	string,
) (string, string, error) {
	return "before\n", "after\n", nil
}

func TestHandlerReturnsStructuredChangesAndContents(t *testing.T) {
	service, _ := testService(t)
	dependencies := reviewDependencies{}
	handler := NewHandler(service, dependencies, dependencies, dependencies)

	changes := httptest.NewRequest(
		http.MethodGet,
		"/workspaces/ws/repositories/repo-1/changes",
		nil,
	)
	changes = changes.WithContext(auth.WithUserID(changes.Context(), "alice"))
	recorder := httptest.NewRecorder()
	handler.Routes().ServeHTTP(recorder, changes)
	var changeBody struct {
		Changes []git.Change `json:"changes"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &changeBody); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusOK || len(changeBody.Changes) != 1 ||
		changeBody.Changes[0].Path != "src/app.ts" {
		t.Fatalf("changes status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	diff := httptest.NewRequest(
		http.MethodGet,
		"/workspaces/ws/repositories/repo-1/diff?path=src%2Fapp.ts",
		nil,
	)
	diff = diff.WithContext(auth.WithUserID(diff.Context(), "alice"))
	recorder = httptest.NewRecorder()
	handler.Routes().ServeHTTP(recorder, diff)
	var diffBody map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &diffBody); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusOK || diffBody["original"] != "before\n" ||
		diffBody["modified"] != "after\n" {
		t.Fatalf("diff status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
