package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"

	"github.com/runspace/runspace/internal/contracts"
)

// ACPClient is the small transport boundary for the Agent Client Protocol.
// Implementations can use stdio JSON-RPC, a socket, or an in-process test peer.
type ACPClient interface {
	Initialize(context.Context) error
	NewSession(context.Context, string) (string, error)
	ResumeSession(context.Context, string, string) error
	SetSessionModel(context.Context, string, string) error
	Prompt(context.Context, string, string) error
	Cancel(context.Context, string) error
	// AnswerPermission resolves a parked permission request. An empty optionID
	// cancels it, matching what the agent sees when a question times out.
	AnswerPermission(context.Context, string, string) error
	Notifications() <-chan ACPNotification
	Close() error
}

// NotificationPermissionRequest marks a notification whose Payload is a
// PermissionRequest rather than streamed agent text.
const NotificationPermissionRequest = "permission_request"

type PermissionOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

// PermissionRequest is a question the agent is blocked on until someone with
// the right grant answers it.
type PermissionRequest struct {
	QuestionID string             `json:"question_id"`
	Title      string             `json:"title"`
	Options    []PermissionOption `json:"options"`
}

type ACPNotification struct {
	SessionID string
	Kind      string
	Text      string
	// Payload carries the raw session/update params so callers can read
	// structured detail (tool calls, permission options) that Text flattens away.
	Payload json.RawMessage
}

type ACPFactory func(context.Context) (ACPClient, error)

type ACP struct {
	factory ACPFactory
	mu      sync.Mutex
	runs    map[string]*acpRun
}

type acpRun struct {
	client  ACPClient
	session string
	prompt  string
	cwd     string
	output  chan contracts.AgentOutput
}

func NewACP(factory ACPFactory) *ACP { return &ACP{factory: factory, runs: make(map[string]*acpRun)} }

var _ contracts.Agent = (*ACP)(nil)
var _ contracts.InputAgent = (*ACP)(nil)

func (a *ACP) Spawn(_ context.Context, req contracts.SpawnRequest) (contracts.AgentRun, error) {
	if strings.TrimSpace(req.RunID) == "" {
		return contracts.AgentRun{}, errors.New("run ID is required")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, exists := a.runs[req.RunID]; exists {
		return contracts.AgentRun{}, errors.New("run already exists")
	}
	a.runs[req.RunID] = &acpRun{prompt: req.Prompt, cwd: req.WorkingDirectory, output: make(chan contracts.AgentOutput, 32)}
	return contracts.AgentRun{ID: req.RunID, Status: contracts.RunQueued}, nil
}

func (a *ACP) Run(ctx context.Context, req contracts.RunRequest) (contracts.AgentRun, error) {
	a.mu.Lock()
	run, ok := a.runs[req.RunID]
	a.mu.Unlock()
	if !ok {
		return contracts.AgentRun{}, errors.New("run not found")
	}
	if a.factory == nil {
		return contracts.AgentRun{}, errors.New("ACP factory is nil")
	}
	client, err := a.factory(ctx)
	if err != nil {
		return contracts.AgentRun{}, err
	}
	if err := client.Initialize(ctx); err != nil {
		_ = client.Close()
		return contracts.AgentRun{}, err
	}
	session, err := client.NewSession(ctx, run.cwd)
	if err != nil {
		_ = client.Close()
		return contracts.AgentRun{}, err
	}
	a.mu.Lock()
	run.client, run.session = client, session
	a.mu.Unlock()
	go a.forward(ctx, req.RunID, run)
	go func() { _ = client.Prompt(ctx, session, run.prompt) }()
	return contracts.AgentRun{ID: req.RunID, Status: contracts.RunRunning}, nil
}

func (a *ACP) forward(ctx context.Context, runID string, run *acpRun) {
	a.mu.Lock()
	client, session := run.client, run.session
	a.mu.Unlock()
	defer close(run.output)
	defer func() { _ = client.Close() }()
	for {
		select {
		case n, ok := <-client.Notifications():
			if !ok {
				return
			}
			if n.SessionID != session || strings.TrimSpace(n.Text) == "" {
				continue
			}
			select {
			case run.output <- contracts.AgentOutput{RunID: runID, Kind: n.Kind, Text: n.Text}:
			case <-ctx.Done():
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (a *ACP) Stop(ctx context.Context, req contracts.StopRequest) error {
	a.mu.Lock()
	run, ok := a.runs[req.RunID]
	a.mu.Unlock()
	if !ok || run.client == nil {
		return nil
	}
	return run.client.Cancel(ctx, run.session)
}

// Send delivers follow-up user input to the active ACP session. ACP models
// conversational input as another session/prompt request, so the existing
// client Prompt method is intentionally reused here.
func (a *ACP) Send(ctx context.Context, req contracts.InputRequest) error {
	if strings.TrimSpace(req.Text) == "" {
		return errors.New("input is required")
	}
	a.mu.Lock()
	run, ok := a.runs[req.RunID]
	if !ok {
		a.mu.Unlock()
		return errors.New("run not found")
	}
	client, session := run.client, run.session
	a.mu.Unlock()
	if client == nil || session == "" {
		return errors.New("ACP session is not active")
	}
	return client.Prompt(ctx, session, req.Text)
}

func (a *ACP) Stream(_ context.Context, req contracts.StreamRequest) (<-chan contracts.AgentOutput, error) {
	a.mu.Lock()
	run, ok := a.runs[req.RunID]
	a.mu.Unlock()
	if !ok {
		return nil, errors.New("run not found")
	}
	return run.output, nil
}
