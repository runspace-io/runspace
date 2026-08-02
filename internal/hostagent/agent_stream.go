package hostagent

import (
	"context"
	"strings"
	"sync"
	"time"

	acpruntime "github.com/runspace/runspace/internal/runtime"
)

// pushTimeout bounds one gateway push. Streaming must not stall behind a slow
// or unreachable gateway: the transcript is already durable on this device, and
// the next push carries the task forward.
const pushTimeout = 5 * time.Second

// tailWindow lets the peer flush chunks it emitted just before session/prompt
// returned, so the last line of a turn is not dropped.
const tailWindow = 250 * time.Millisecond

type taskStreamUpdate struct {
	WorkspaceID string                `json:"workspace_id"`
	ThreadID    string                `json:"thread_id"`
	AgentID     string                `json:"agent_id"`
	ResourceID  string                `json:"resource_id"`
	Title       string                `json:"title"`
	Status      string                `json:"status"`
	Messages    []LocalSessionMessage `json:"messages"`
	Question    *LocalPendingQuestion `json:"question,omitempty"`
}

// pushTaskUpdate mirrors one streamed step to the gateway so grantees and
// reloaded browsers can read it. Failures are intentionally swallowed by
// callers: a device that cannot reach its gateway must still run its own agent.
func (s *Server) pushTaskUpdate(
	ctx context.Context, session *agentSession, messages []LocalSessionMessage, status string,
) error {
	return s.pushTaskEvent(ctx, session, messages, status, nil)
}

func (s *Server) pushTaskEvent(
	ctx context.Context, session *agentSession, messages []LocalSessionMessage,
	status string, question *LocalPendingQuestion,
) error {
	s.mu.RLock()
	user := s.userConfigLockedReadOnly(session.userID)
	binding, found := user.Resources[session.resourceID]
	title := user.Sessions[session.publicID].Title
	s.mu.RUnlock()
	gatewayURL := strings.TrimRight(strings.TrimSpace(binding.GatewayURL), "/")
	if !found || gatewayURL == "" || strings.TrimSpace(binding.WorkspaceID) == "" ||
		strings.TrimSpace(title) == "" {
		return nil
	}
	timed, cancel := context.WithTimeout(ctx, pushTimeout)
	defer cancel()
	return s.gateway(
		timed, gatewayURL+"/users/me/agent-tasks/"+session.publicID+"/events", session.userID,
		taskStreamUpdate{
			WorkspaceID: binding.WorkspaceID, ThreadID: session.threadID,
			AgentID: session.agentID, ResourceID: session.resourceID,
			Title: title, Status: status, Messages: messages, Question: question,
		}, nil,
	)
}

type turnOutcome struct {
	outputs  []agentPromptOutput
	status   string
	question *LocalPendingQuestion
}

// turnState is shared between the goroutine consuming chunks and the request
// that may return early, so a parked turn can still report what it produced.
type turnState struct {
	mu       sync.Mutex
	outputs  []agentPromptOutput
	question *LocalPendingQuestion
	parked   chan struct{}
	once     sync.Once
}

func newTurnState() *turnState {
	return &turnState{outputs: make([]agentPromptOutput, 0, 8), parked: make(chan struct{})}
}

func (t *turnState) addOutput(output agentPromptOutput) {
	t.mu.Lock()
	t.outputs = append(t.outputs, output)
	t.mu.Unlock()
}

func (t *turnState) park(question LocalPendingQuestion) {
	t.mu.Lock()
	t.question = &question
	t.mu.Unlock()
	t.once.Do(func() { close(t.parked) })
}

func (t *turnState) snapshot() ([]agentPromptOutput, *LocalPendingQuestion) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]agentPromptOutput(nil), t.outputs...), t.question
}

// streamTurn runs one prompt, persisting and forwarding each chunk as it
// arrives. It returns when the turn ends, or as soon as the agent parks a
// question — a human may take minutes to answer, and the caller's HTTP request
// must not be held open for that. In the parked case the turn keeps running in
// the background and finishes its own bookkeeping.
func (s *Server) streamTurn(
	ctx context.Context, session *agentSession, prompt string,
) (turnOutcome, error) {
	session.mu.Lock()
	state := newTurnState()
	chunks := make(chan acpruntime.ACPNotification, 256)
	stop := make(chan struct{})
	consumed := make(chan struct{})
	finished := make(chan error, 1)
	// Detached once for the whole turn. A parked question returns this request
	// early, which cancels its context; anything still streaming would then fail
	// every push and silently truncate the transcript the gateway serves.
	detached := context.WithoutCancel(ctx)
	go collectSessionNotifications(session, chunks, stop)
	go func() {
		defer close(consumed)
		s.consumeTurn(detached, session, chunks, state)
	}()
	go s.runTurn(detached, session, prompt, state, turnPlumbing{
		stop: stop, consumed: consumed, finished: finished,
	})
	select {
	case err := <-finished:
		outputs, question := state.snapshot()
		return turnOutcome{outputs: outputs, status: turnStatus(err), question: question}, err
	case <-state.parked:
		outputs, question := state.snapshot()
		return turnOutcome{outputs: outputs, status: "waiting_approval", question: question}, nil
	}
}

type turnPlumbing struct {
	stop     chan struct{}
	consumed chan struct{}
	finished chan error
}

// runTurn owns the turn from prompt to terminal status and releases the session
// lock when it is genuinely over, whether or not the request already returned.
// ctx must outlive the request (see streamTurn).
func (s *Server) runTurn(
	ctx context.Context, session *agentSession, prompt string,
	state *turnState, plumbing turnPlumbing,
) {
	err := session.client.Prompt(ctx, session.nativeID, prompt)
	timer := time.NewTimer(tailWindow)
	<-timer.C
	close(plumbing.stop)
	<-plumbing.consumed
	status := sessionTurnStatus(session, err)
	s.clearSessionQuestion(session.publicID)
	_ = s.setSessionStatus(session.publicID, status)
	session.cancelled.Store(false)
	session.mu.Unlock()
	_ = s.pushTaskUpdate(ctx, session, nil, status)
	plumbing.finished <- err
}

// sessionTurnStatus reports an operator stop as "cancelled". Cancelling aborts
// the prompt, so the raw error would otherwise surface as a failure.
func sessionTurnStatus(session *agentSession, err error) string {
	if session.cancelled.Load() {
		return "cancelled"
	}
	return turnStatus(err)
}

func turnStatus(err error) string {
	if err != nil {
		return "failed"
	}
	return "completed"
}

func (s *Server) consumeTurn(
	ctx context.Context, session *agentSession, chunks <-chan acpruntime.ACPNotification,
	state *turnState,
) {
	// One unreachable gateway should cost one timeout, not one per chunk, so a
	// failed push stops forwarding for the rest of the turn. The transcript is
	// still durable locally and the terminal status push retries the connection.
	degraded := false
	for chunk := range chunks {
		if chunk.Kind == acpruntime.NotificationPermissionRequest {
			if s.parkQuestion(ctx, session, chunk, state) != nil {
				degraded = true
			}
			continue
		}
		text := strings.TrimSpace(chunk.Text)
		if text == "" {
			continue
		}
		state.addOutput(agentPromptOutput{Kind: chunk.Kind, Text: text})
		message, err := s.appendSessionMessage(session.publicID, "agent", text, "running")
		if err != nil || message.ID == "" || degraded {
			continue
		}
		if s.pushTaskUpdate(ctx, session, []LocalSessionMessage{message}, "running") != nil {
			degraded = true
		}
	}
}
