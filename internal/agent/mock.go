package agent

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/runspace/runspace/internal/contracts"
)

var ErrRunNotFound = errors.New("agent run not found")

type run struct {
	id      string
	prompt  string
	mu      sync.Mutex
	state   contracts.RunState
	outputs chan contracts.AgentOutput
	cancel  context.CancelFunc
}

// MockRuntime is deterministic and useful for UI development and contract tests.
// It models a real runtime's queued/running/terminal lifecycle.
type MockRuntime struct {
	mu   sync.RWMutex
	runs map[string]*run
}

func NewMockRuntime() *MockRuntime { return &MockRuntime{runs: make(map[string]*run)} }

var _ contracts.Agent = (*MockRuntime)(nil)

func (m *MockRuntime) Spawn(_ context.Context, req contracts.SpawnRequest) (contracts.AgentRun, error) {
	if req.RunID == "" {
		return contracts.AgentRun{}, errors.New("run ID is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.runs[req.RunID]; exists {
		return contracts.AgentRun{}, fmt.Errorf("run %q already exists", req.RunID)
	}
	m.runs[req.RunID] = &run{id: req.RunID, prompt: req.Prompt, state: contracts.RunQueued, outputs: make(chan contracts.AgentOutput, 16)}
	return contracts.AgentRun{ID: req.RunID, Status: contracts.RunQueued}, nil
}

func (m *MockRuntime) Run(ctx context.Context, req contracts.RunRequest) (contracts.AgentRun, error) {
	r, err := m.get(req.RunID)
	if err != nil {
		return contracts.AgentRun{}, err
	}
	r.mu.Lock()
	if r.state != contracts.RunQueued {
		state := r.state
		r.mu.Unlock()
		return contracts.AgentRun{ID: r.id, Status: state}, fmt.Errorf("run %q cannot start from %s", r.id, state)
	}
	r.state = contracts.RunRunning
	runCtx, cancel := context.WithCancel(ctx)
	r.cancel = cancel
	r.mu.Unlock()
	go m.execute(runCtx, r)
	return contracts.AgentRun{ID: r.id, Status: contracts.RunRunning}, nil
}

func (m *MockRuntime) execute(ctx context.Context, r *run) {
	m.emit(ctx, r, "status", "agent started")
	m.emit(ctx, r, "output", r.prompt)
	r.mu.Lock()
	if r.state == contracts.RunRunning {
		r.state = contracts.RunSucceeded
		m.emitLocked(r, "status", "agent finished")
	}
	close(r.outputs)
	r.mu.Unlock()
}

func (m *MockRuntime) Stop(_ context.Context, req contracts.StopRequest) error {
	r, err := m.get(req.RunID)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state == contracts.RunQueued {
		r.state = contracts.RunCancelled
		close(r.outputs)
		return nil
	}
	if r.state != contracts.RunRunning {
		return nil
	}
	r.state = contracts.RunStopping
	if r.cancel != nil {
		r.cancel()
	}
	r.state = contracts.RunCancelled
	return nil
}

func (m *MockRuntime) Stream(_ context.Context, req contracts.StreamRequest) (<-chan contracts.AgentOutput, error) {
	r, err := m.get(req.RunID)
	if err != nil {
		return nil, err
	}
	return r.outputs, nil
}

func (m *MockRuntime) Send(_ context.Context, req contracts.InputRequest) error {
	r, err := m.get(req.RunID)
	if err != nil {
		return err
	}
	if req.Text == "" {
		return errors.New("input is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state != contracts.RunRunning {
		return fmt.Errorf("run %q is not running", req.RunID)
	}
	m.emitLocked(r, "input", req.Text)
	return nil
}

func (m *MockRuntime) get(id string) (*run, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.runs[id]
	if !ok {
		return nil, ErrRunNotFound
	}
	return r, nil
}
func (m *MockRuntime) emit(ctx context.Context, r *run, kind, text string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m.emitLockedContext(ctx, r, kind, text)
}
func (m *MockRuntime) emitLocked(r *run, kind, text string) {
	r.outputs <- contracts.AgentOutput{RunID: r.id, Kind: kind, Text: text}
}
func (m *MockRuntime) emitLockedContext(ctx context.Context, r *run, kind, text string) {
	select {
	case r.outputs <- contracts.AgentOutput{RunID: r.id, Kind: kind, Text: text}:
	case <-ctx.Done():
	}
}
