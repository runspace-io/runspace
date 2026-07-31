package runtime

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os/exec"
	"strings"
	"sync"

	"github.com/runspace/runspace/internal/contracts"
)

type CLI struct{}

func (CLI) Run(ctx context.Context, dir string, args ...string) (string, error) {
	c := exec.CommandContext(ctx, "codex", args...)
	c.Dir = dir
	b, e := c.CombinedOutput()
	return string(b), e
}

type process interface {
	Wait() error
	Kill() error
	StdoutPipe() (io.ReadCloser, error)
	StderrPipe() (io.ReadCloser, error)
	Start() error
}

type processFactory func(context.Context, string, ...string) process

type Codex struct {
	runner     CommandRunner
	newProcess processFactory
	state      *codexState
}

type codexState struct {
	mu        sync.Mutex
	processes map[string]process
}

func NewCodex(r CommandRunner) Codex {
	return Codex{runner: r, newProcess: newCodexProcess, state: &codexState{processes: make(map[string]process)}}
}

func newCodexProcess(ctx context.Context, dir string, args ...string) process {
	command := exec.CommandContext(ctx, "codex", args...)
	command.Dir = dir
	return execProcess{command: command}
}

type execProcess struct{ command *exec.Cmd }

func (p execProcess) Wait() error                        { return p.command.Wait() }
func (p execProcess) Start() error                       { return p.command.Start() }
func (p execProcess) StdoutPipe() (io.ReadCloser, error) { return p.command.StdoutPipe() }
func (p execProcess) StderrPipe() (io.ReadCloser, error) { return p.command.StderrPipe() }
func (p execProcess) Kill() error {
	if p.command.Process == nil {
		return nil
	}
	return p.command.Process.Kill()
}

var _ contracts.Agent = (*Codex)(nil)

func (c Codex) Spawn(_ context.Context, r contracts.SpawnRequest) (contracts.AgentRun, error) {
	if r.RunID == "" {
		return contracts.AgentRun{}, errors.New("run ID required")
	}
	return contracts.AgentRun{ID: r.RunID, Status: contracts.RunQueued}, nil
}
func (c Codex) Run(ctx context.Context, r contracts.RunRequest) (contracts.AgentRun, error) {
	if c.runner == nil {
		return contracts.AgentRun{}, errors.New("runner is nil")
	}
	if strings.TrimSpace(r.RunID) == "" {
		return contracts.AgentRun{}, errors.New("run ID required")
	}
	go func() { _, _ = c.runner.Run(ctx, "", "exec", r.RunID) }()
	return contracts.AgentRun{ID: r.RunID, Status: contracts.RunRunning}, nil
}

func (c Codex) Stop(_ context.Context, request contracts.StopRequest) error {
	if c.state == nil {
		return nil
	}
	c.state.mu.Lock()
	process, ok := c.state.processes[request.RunID]
	delete(c.state.processes, request.RunID)
	c.state.mu.Unlock()
	if !ok {
		return nil
	}
	return process.Kill()
}

func (Codex) Send(context.Context, contracts.InputRequest) error {
	return errors.New("codex input requires an interactive supervisor")
}

func (c Codex) Stream(ctx context.Context, request contracts.StreamRequest) (<-chan contracts.AgentOutput, error) {
	if strings.TrimSpace(request.RunID) == "" {
		return nil, errors.New("run ID required")
	}
	process := c.newProcess(ctx, "", "exec", request.RunID)
	stdout, err := process.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := process.StderrPipe()
	if err != nil {
		_ = stdout.Close()
		return nil, err
	}
	if err := process.Start(); err != nil {
		_ = stdout.Close()
		_ = stderr.Close()
		return nil, err
	}
	if c.state == nil {
		return nil, errors.New("process state is nil")
	}
	c.state.mu.Lock()
	c.state.processes[request.RunID] = process
	c.state.mu.Unlock()
	output := make(chan contracts.AgentOutput, 32)
	go streamAgentProcess(ctx, request.RunID, process, stdout, stderr, output)
	return output, nil
}

func streamAgentProcess(ctx context.Context, runID string, process process, stdout, stderr io.ReadCloser, output chan<- contracts.AgentOutput) {
	defer close(output)
	defer func() { _ = stdout.Close() }()
	defer func() { _ = stderr.Close() }()
	lines := make(chan string, 32)
	var readers sync.WaitGroup
	readers.Add(2)
	go func() { defer readers.Done(); readLines(stdout, lines) }()
	go func() { defer readers.Done(); readLines(stderr, lines) }()
	go func() { readers.Wait(); close(lines) }()
	for {
		select {
		case line, ok := <-lines:
			if !ok {
				_ = process.Wait()
				return
			}
			select {
			case output <- contracts.AgentOutput{RunID: runID, Kind: "output", Text: line}:
			case <-ctx.Done():
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func readLines(reader io.Reader, lines chan<- string) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		lines <- scanner.Text()
	}
}
