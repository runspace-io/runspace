package publish

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/runspace/runspace/internal/git"
)

func TestPublishUsesNativeGitAndGitHubPRAPI(t *testing.T) {
	root := t.TempDir()
	bare := filepath.Join(root, "remote.git")
	work := filepath.Join(root, "work")
	runGit(t, root, "init", "--bare", bare)
	runGit(t, root, "init", "-b", "main", work)
	runGit(t, work, "config", "user.email", "test@runspace.local")
	runGit(t, work, "config", "user.name", "Runspace Test")
	if err := os.WriteFile(filepath.Join(work, "README.md"), []byte("base\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	runGit(t, work, "add", "README.md")
	runGit(t, work, "commit", "-m", "initial")
	runGit(t, work, "remote", "add", "origin", bare)
	runGit(t, work, "push", "-u", "origin", "main")
	if err := os.WriteFile(filepath.Join(work, "README.md"), []byte("changed\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Path string
		Body map[string]string
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		got.Path = request.URL.Path
		_ = json.NewDecoder(request.Body).Decode(&got.Body)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"number":7,"html_url":"http://github.local/pr/7"}`))
	}))
	defer server.Close()
	service := New(git.NewProvider(), git.GitHubRemote{BaseURL: server.URL, Token: "test-token"})
	result, err := service.Publish(context.Background(), Request{ID: "run-1", RepositoryPath: work, Repository: "acme/repo", Branch: "runspace/run-1", Base: "main", CommitMessage: "docs: update readme", Title: "Update README", Body: "Automated change"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Branch.Name != "runspace/run-1" || result.PullRequest.Number != 7 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if got.Path != "/repos/acme/repo/pulls" || got.Body["head"] != "runspace/run-1" || got.Body["base"] != "main" {
		t.Fatalf("unexpected PR request path=%q body=%v", got.Path, got.Body)
	}
	if !strings.Contains(runGit(t, root, "--git-dir", bare, "show", "runspace/run-1:README.md"), "changed") {
		t.Fatal("published branch does not contain committed change")
	}
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return string(output)
}
