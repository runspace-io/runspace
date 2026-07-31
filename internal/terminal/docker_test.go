package terminal

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDockerArgsRetainIsolation(t *testing.T) {
	args := dockerArgs("agent:test", "/repo", "printf hello")
	joined := strings.Join(args, " ")
	for _, expected := range []string{"--network none", "--cpus 2", "--memory 4g", "--pids-limit 256", "--cap-drop ALL", "--security-opt no-new-privileges", "-w /workspace", "sh -lc printf hello"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("docker args missing %q: %s", expected, joined)
		}
	}
}

func TestDockerFactoryCanUseNamedRepositoryVolume(t *testing.T) {
	args := dockerArgsWithVolume("agent:test", "repository-data", "/var/lib/runspace/repositories/ws/repo", "sh")
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "repository-data:/var/lib/runspace/repositories:rw") || !strings.Contains(joined, "-w /var/lib/runspace/repositories/ws/repo") {
		t.Fatalf("volume args=%s", joined)
	}
}

func TestDockerFactoryRejectsUnsafeRoot(t *testing.T) {
	factory := NewDockerFactory("agent:test")
	_, err := factory.Open(context.Background(), OpenRequest{WorkspaceID: "w", RepositoryID: "r", Root: filepath.Join(t.TempDir(), "missing"), Command: "sh"})
	if err == nil {
		t.Fatal("expected missing root rejection")
	}
}

//nolint:cyclop // Contract test intentionally exercises the complete session lifecycle.
func TestProcessSessionStreamsOutputAndInput(t *testing.T) {
	process := newFakeProcess()
	session, err := newProcessSession(context.Background(), process)
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Input([]byte("echo\n")); err != nil {
		t.Fatal(err)
	}
	assertInput(t, process.input, "echo\n")
	process.emit([]byte("ready\n"))
	assertOutput(t, session.Output(), "ready\n")
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	if err := session.Input([]byte("after")); !errors.Is(err, ErrClosed) {
		t.Fatalf("expected closed input, got %v", err)
	}
}

func assertInput(t *testing.T, input <-chan []byte, expected string) {
	t.Helper()
	select {
	case got := <-input:
		if string(got) != expected {
			t.Fatalf("input=%q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("terminal input was not delivered")
	}
}

func assertOutput(t *testing.T, output <-chan []byte, expected string) {
	t.Helper()
	select {
	case got := <-output:
		if string(got) != expected {
			t.Fatalf("output=%q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("terminal output was not delivered")
	}
}

type fakeProcess struct {
	stdin  *fakeWriteCloser
	stdout *fakeReadCloser
	stderr *fakeReadCloser
	input  chan []byte
	done   chan struct{}
	once   sync.Once
}

func newFakeProcess() *fakeProcess {
	return &fakeProcess{stdin: newFakeWriteCloser(), stdout: newFakeReadCloser(), stderr: newFakeReadCloser(), input: make(chan []byte, 1), done: make(chan struct{})}
}
func (p *fakeProcess) Start() error { return nil }
func (p *fakeProcess) Wait() error  { <-p.done; return nil }
func (p *fakeProcess) Kill() error {
	p.once.Do(func() { close(p.done); _ = p.stdout.Close(); _ = p.stderr.Close() })
	return nil
}
func (p *fakeProcess) StdinPipe() (io.WriteCloser, error) {
	p.stdin.onWrite = func(data []byte) { p.input <- data }
	return p.stdin, nil
}
func (p *fakeProcess) StdoutPipe() (io.ReadCloser, error) { return p.stdout, nil }
func (p *fakeProcess) StderrPipe() (io.ReadCloser, error) { return p.stderr, nil }
func (p *fakeProcess) emit(data []byte)                   { p.stdout.emit(data) }

type fakeWriteCloser struct{ onWrite func([]byte) }

func newWriteCloser() *fakeWriteCloser     { return &fakeWriteCloser{} }
func newFakeWriteCloser() *fakeWriteCloser { return newWriteCloser() }
func (w *fakeWriteCloser) Write(data []byte) (int, error) {
	if w.onWrite != nil {
		w.onWrite(append([]byte(nil), data...))
	}
	return len(data), nil
}
func (*fakeWriteCloser) Close() error { return nil }

type fakeReadCloser struct {
	reader *os.File
	writer *os.File
	once   sync.Once
}

func newFakeReadCloser() *fakeReadCloser {
	reader, writer, _ := os.Pipe()
	return &fakeReadCloser{reader: reader, writer: writer}
}
func (r *fakeReadCloser) Read(data []byte) (int, error) { return r.reader.Read(data) }
func (r *fakeReadCloser) Close() error {
	r.once.Do(func() { _ = r.reader.Close(); _ = r.writer.Close() })
	return nil
}
func (r *fakeReadCloser) emit(data []byte) { _, _ = r.writer.Write(data) }
