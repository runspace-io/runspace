package runs

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/runspace/runspace/internal/contracts"
	"github.com/runspace/runspace/internal/events"
)

type Run struct {
	ID               string             `json:"id"`
	WorkspaceID      string             `json:"workspace_id"`
	ThreadID         string             `json:"thread_id,omitempty"`
	ChannelID        string             `json:"channel_id,omitempty"`
	RepositoryID     string             `json:"repository_id"`
	Prompt           string             `json:"prompt"`
	AgentCommand     string             `json:"agent_command,omitempty"`
	WorkingDirectory string             `json:"-"`
	Status           contracts.RunState `json:"status"`
	CreatedAt        time.Time          `json:"created_at"`
	UpdatedAt        time.Time          `json:"updated_at"`
}
type Output struct {
	ID        string    `json:"id"`
	RunID     string    `json:"run_id"`
	Kind      string    `json:"kind"`
	Text      string    `json:"text"`
	Sequence  uint64    `json:"sequence"`
	CreatedAt time.Time `json:"created_at"`
}
type Store interface {
	SaveRun(context.Context, Run) error
	GetRun(context.Context, string) (Run, error)
	SaveOutput(context.Context, Output) error
	ListOutputs(context.Context, string) ([]Output, error)
	ListRuns(context.Context, string) ([]Run, error)
}
type Service struct {
	mu           sync.RWMutex
	runs         map[string]Run
	agent        contracts.Agent
	agentFactory func(string) contracts.Agent
	runAgents    map[string]contracts.Agent
	outputSeq    map[string]uint64
	runCancels   map[string]context.CancelFunc
	runContexts  map[string]context.Context
	publisher    events.Publisher
	now          func() time.Time
	store        Store
}

func (s *Service) SetStore(store Store) { s.mu.Lock(); defer s.mu.Unlock(); s.store = store }

func New(a contracts.Agent, p events.Publisher) *Service {
	return &Service{runs: map[string]Run{}, runAgents: map[string]contracts.Agent{}, outputSeq: map[string]uint64{}, runCancels: map[string]context.CancelFunc{}, runContexts: map[string]context.Context{}, agent: a, publisher: p, now: time.Now}
}
func (s *Service) SetAgentFactory(factory func(string) contracts.Agent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agentFactory = factory
}
func (s *Service) Create(ctx context.Context, r contracts.SpawnRequest) (Run, error) {
	if r.RunID == "" || r.WorkspaceID == "" {
		return Run{}, errors.New("run ID and workspace ID required")
	}
	s.mu.Lock()
	if _, ok := s.runs[r.RunID]; ok {
		s.mu.Unlock()
		return Run{}, errors.New("run already exists")
	}
	now := s.now().UTC()
	run := Run{ID: r.RunID, WorkspaceID: r.WorkspaceID, ThreadID: r.ThreadID, ChannelID: r.ChannelID, RepositoryID: r.Repository, Prompt: r.Prompt, AgentCommand: r.AgentCommand, WorkingDirectory: r.WorkingDirectory, Status: contracts.RunQueued, CreatedAt: now, UpdatedAt: now}
	s.runs[r.RunID] = run
	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	s.runCancels[r.RunID] = cancel
	s.runContexts[r.RunID] = runCtx
	agent := s.agent
	if s.agentFactory != nil {
		agent = s.agentFactory(r.AgentCommand)
	}
	s.runAgents[r.RunID] = agent
	s.mu.Unlock()
	if s.store != nil {
		_ = s.store.SaveRun(ctx, run)
	}
	s.publish(ctx, "run.requested", run)
	if agent != nil {
		if _, err := agent.Spawn(ctx, r); err != nil {
			return Run{}, err
		}
	}
	return run, nil
}
func (s *Service) Get(ctx context.Context, id string) (Run, error) {
	s.mu.RLock()
	r, ok := s.runs[id]
	store := s.store
	s.mu.RUnlock()
	if !ok {
		if store != nil {
			run, err := store.GetRun(ctx, id)
			if err == nil {
				s.mu.Lock()
				s.runs[id] = run
				s.mu.Unlock()
			}
			return run, err
		}
		return Run{}, errors.New("run not found")
	}
	return r, nil
}
func (s *Service) Start(ctx context.Context, id string) (Run, error) {
	s.mu.Lock()
	r, ok := s.runs[id]
	if !ok {
		s.mu.Unlock()
		return Run{}, errors.New("run not found")
	}
	if err := contracts.TransitionRun(r.Status, contracts.RunRunning); err != nil {
		s.mu.Unlock()
		return Run{}, err
	}
	r.Status = contracts.RunRunning
	r.UpdatedAt = s.now().UTC()
	s.runs[id] = r
	s.mu.Unlock()
	if s.store != nil {
		_ = s.store.SaveRun(ctx, r)
	}
	runCtx := s.runContext(id, ctx)
	s.publish(ctx, "run.started", r)
	agent := s.agentFor(id)
	if agent != nil {
		if _, err := agent.Run(runCtx, contracts.RunRequest{RunID: id}); err != nil {
			return Run{}, err
		}
		if output, err := agent.Stream(runCtx, contracts.StreamRequest{RunID: id}); err == nil {
			go s.consume(runCtx, id, output)
		}
	}
	return r, nil
}

func (s *Service) consume(ctx context.Context, id string, output <-chan contracts.AgentOutput) {
	for item := range output {
		run, err := s.Get(ctx, id)
		if err != nil {
			return
		}
		s.publishOutput(ctx, run, item)
	}
	s.mu.Lock()
	run, ok := s.runs[id]
	if ok && run.Status == contracts.RunRunning {
		run.Status = contracts.RunSucceeded
		run.UpdatedAt = s.now().UTC()
		s.runs[id] = run
	}
	if ok && s.store != nil {
		_ = s.store.SaveRun(ctx, run)
	}
	s.mu.Unlock()
	if ok {
		s.publish(ctx, "run.finished", run)
	}
	s.clearRunContext(id)
}

func (s *Service) runContext(id string, fallback context.Context) context.Context {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if runCtx, ok := s.runContexts[id]; ok {
		return runCtx
	}
	return context.WithoutCancel(fallback)
}

func (s *Service) clearRunContext(id string) {
	s.mu.Lock()
	if cancel, ok := s.runCancels[id]; ok {
		cancel()
		delete(s.runCancels, id)
	}
	delete(s.runContexts, id)
	s.mu.Unlock()
}

func (s *Service) publishOutput(ctx context.Context, run Run, output contracts.AgentOutput) {
	s.mu.Lock()
	s.outputSeq[run.ID]++
	sequence := s.outputSeq[run.ID]
	now := s.now().UTC()
	s.mu.Unlock()
	payload := struct {
		contracts.AgentOutput
		ThreadID  string `json:"thread_id,omitempty"`
		ChannelID string `json:"channel_id,omitempty"`
	}{AgentOutput: output, ThreadID: run.ThreadID, ChannelID: run.ChannelID}
	eventID := fmt.Sprintf("%s-output-%s-%d", run.ID, output.Kind, sequence)
	if s.store != nil {
		_ = s.store.SaveOutput(ctx, Output{ID: eventID, RunID: run.ID, Kind: output.Kind, Text: output.Text, Sequence: sequence, CreatedAt: now})
	}
	if s.publisher == nil {
		return
	}
	event, err := contracts.NewEvent(eventID, "agent.output", run.WorkspaceID, "agent-"+run.ID, "agent", payload, run.UpdatedAt)
	if err == nil {
		_ = s.publisher.Publish(ctx, event)
	}
}
func (s *Service) Stop(ctx context.Context, id string) (Run, error) {
	s.mu.Lock()
	r, ok := s.runs[id]
	if !ok {
		s.mu.Unlock()
		return Run{}, errors.New("run not found")
	}
	if r.Status == contracts.RunRunning {
		r.Status = contracts.RunStopping
		r.UpdatedAt = s.now().UTC()
		s.runs[id] = r
	}
	s.mu.Unlock()
	if s.store != nil {
		_ = s.store.SaveRun(ctx, r)
	}
	s.publish(ctx, "run.stop_requested", r)
	if agent := s.agentFor(id); agent != nil {
		if cancel := s.cancelRun(id); cancel != nil {
			cancel()
		}
		_ = agent.Stop(context.WithoutCancel(ctx), contracts.StopRequest{RunID: id})
	}
	s.mu.Lock()
	r = s.runs[id]
	if r.Status == contracts.RunStopping {
		r.Status = contracts.RunCancelled
		r.UpdatedAt = s.now().UTC()
		s.runs[id] = r
	}
	s.mu.Unlock()
	if s.store != nil {
		_ = s.store.SaveRun(ctx, r)
	}
	s.publish(ctx, "run.stopped", r)
	return r, nil
}

func (s *Service) cancelRun(id string) context.CancelFunc {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.runCancels[id]
}
