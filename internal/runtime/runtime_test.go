package runtime

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/runspace/runspace/internal/contracts"
)

type recordingRunner struct{ calls [][]string }

type fakeProcess struct {
	stdout io.ReadCloser
	stderr io.ReadCloser
}

func (p *fakeProcess) Wait() error                        { return nil }
func (p *fakeProcess) Kill() error                        { return nil }
func (p *fakeProcess) Start() error                       { return nil }
func (p *fakeProcess) StdoutPipe() (io.ReadCloser, error) { return p.stdout, nil }
func (p *fakeProcess) StderrPipe() (io.ReadCloser, error) { return p.stderr, nil }

func (r *recordingRunner) Run(_ context.Context, _ string, args ...string) (string, error) {
	r.calls = append(r.calls, args)
	return "", nil
}

func TestSafeRunWorkspaceRejectsTraversal(t *testing.T) {
	if _, err := SafeRunWorkspace(t.TempDir(), "../escape"); err == nil {
		t.Fatal("expected traversal rejection")
	}
	path, err := SafeRunWorkspace(t.TempDir(), "run-1")
	if err != nil || !strings.Contains(path, "run-1") {
		t.Fatalf("path=%q err=%v", path, err)
	}
}

func TestDockerExecutorAppliesIsolationLimits(t *testing.T) {
	runner := &recordingRunner{}
	executor := NewDockerExecutor(runner, "agent:test")
	if err := executor.Run(context.Background(), t.TempDir(), "run-1"); err != nil {
		t.Fatal(err)
	}
	call := strings.Join(runner.calls[0], " ")
	for _, expected := range []string{"--network none", "--cpus 2", "--memory 4g", "--pids-limit 256"} {
		if !strings.Contains(call, expected) {
			t.Fatalf("docker command %q missing %q", call, expected)
		}
	}
}

func TestDockerExecutorRejectsUnsafeRunWorkspace(t *testing.T) {
	runner := &recordingRunner{}
	executor := NewDockerExecutor(runner, "agent:test")
	if err := executor.Run(context.Background(), t.TempDir(), "../escape"); err == nil {
		t.Fatal("expected unsafe workspace rejection")
	}
	if len(runner.calls) != 0 {
		t.Fatal("docker must not run for unsafe workspace")
	}
}

func TestCodexConformsToAgentContract(t *testing.T) {
	var _ contracts.Agent = Codex{}
	runner := &recordingRunner{}
	client := NewCodex(runner)
	spawned, err := client.Spawn(context.Background(), contracts.SpawnRequest{RunID: "run-1"})
	if err != nil || spawned.Status != contracts.RunQueued {
		t.Fatalf("spawned=%+v err=%v", spawned, err)
	}
	run, err := client.Run(context.Background(), contracts.RunRequest{RunID: "run-1"})
	if err != nil || run.Status != contracts.RunRunning {
		t.Fatalf("run=%+v err=%v", run, err)
	}
}

func TestCodexStreamNormalizesStdoutAndStderr(t *testing.T) {
	client := NewCodex(&recordingRunner{})
	client.newProcess = func(context.Context, string, ...string) process {
		return &fakeProcess{stdout: io.NopCloser(strings.NewReader("out\n")), stderr: io.NopCloser(strings.NewReader("err\n"))}
	}
	output, err := client.Stream(context.Background(), contracts.StreamRequest{RunID: "run-1"})
	if err != nil {
		t.Fatal(err)
	}
	lines := make([]string, 0, 2)
	for item := range output {
		lines = append(lines, item.Text)
	}
	if len(lines) != 2 || !containsLine(lines, "out") || !containsLine(lines, "err") {
		t.Fatalf("unexpected output: %#v", lines)
	}
}

func containsLine(lines []string, expected string) bool {
	for _, line := range lines {
		if line == expected {
			return true
		}
	}
	return false
}
