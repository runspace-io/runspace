package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/runspace/runspace/internal/contracts"
)

func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v (%s)", args, err, out)
	}
	return string(out)
}

//nolint:cyclop // The integration test intentionally exercises the complete Git lifecycle.
func TestProviderTempRepositoryLifecycle(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatal(err)
	}
	gitRun(t, source, "init", "--initial-branch=main")
	gitRun(t, source, "config", "user.email", "test@example.com")
	gitRun(t, source, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, source, "add", ".")
	gitRun(t, source, "commit", "-m", "initial")
	dest := filepath.Join(root, "clone")
	p := NewProvider()
	result, err := p.Clone(ctx, contracts.CloneRequest{RepositoryURL: source, Destination: dest, Ref: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Path != dest {
		t.Fatalf("path = %q", result.Path)
	}
	if _, err := p.CreateBranch(ctx, contracts.BranchRequest{Repository: dest, Name: "agent/change", From: "main"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "change.txt"), []byte("change\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	status, err := p.Status(ctx, dest)
	if err != nil || !strings.Contains(status, "change.txt") {
		t.Fatalf("status=%q err=%v", status, err)
	}
	if _, err := p.Commit(ctx, dest, "agent change"); err != nil {
		t.Fatal(err)
	}
	if err := p.Push(ctx, dest, "", "agent/change"); err != nil {
		t.Fatalf("push: %v", err)
	}
}

type recordingRunner struct{ calls [][]string }

func (r *recordingRunner) Run(_ context.Context, _ string, args ...string) (string, error) {
	r.calls = append(r.calls, args)
	if len(args) > 0 && args[0] == "rev-parse" {
		return "abc\n", nil
	}
	return "", nil
}

func TestProviderUsesArgumentArrays(t *testing.T) {
	r := &recordingRunner{}
	p := NewProviderWithRunner(r)
	_, err := p.CreateBranch(context.Background(), contracts.BranchRequest{Repository: t.TempDir(), Name: "safe;touch", From: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(r.calls[0], " "); got != "switch -c safe;touch main" {
		t.Fatalf("args = %q", got)
	}
}
