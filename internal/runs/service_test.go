package runs

import (
	"context"
	"testing"
	"time"

	"github.com/runspace/runspace/internal/agent"
	"github.com/runspace/runspace/internal/contracts"
)

type eventPublisher struct{ events []contracts.EventEnvelope }

func (p *eventPublisher) Publish(_ context.Context, event contracts.EventEnvelope) error {
	p.events = append(p.events, event)
	return nil
}

func TestLifecycle(t *testing.T) {
	a := agent.NewMockRuntime()
	s := New(a, nil)
	r, e := s.Create(context.Background(), contracts.SpawnRequest{RunID: "r", WorkspaceID: "w", Prompt: "x"})
	if e != nil || r.Status != contracts.RunQueued {
		t.Fatal(r, e)
	}
	r, e = s.Start(context.Background(), "r")
	if e != nil || r.Status != contracts.RunRunning {
		t.Fatal(r, e)
	}
	r, e = s.Stop(context.Background(), "r")
	if e != nil || r.Status != contracts.RunCancelled {
		t.Fatal(r, e)
	}
}
func TestRetry(t *testing.T) {
	s := New(nil, nil)
	original := contracts.SpawnRequest{
		RunID: "old", WorkspaceID: "w", ThreadID: "thread", ChannelID: "channel",
		Repository: "repo", Prompt: "fix it", AgentCommand: "codex-acp",
		WorkingDirectory: "/workspace/repo",
	}
	old, e := s.Create(context.Background(), original)
	if e != nil {
		t.Fatal(e)
	}
	r, e := s.Retry(context.Background(), "old", "new")
	if e != nil {
		t.Fatal(r, e)
	}
	expected := old
	expected.ID = "new"
	expected.CreatedAt = r.CreatedAt
	expected.UpdatedAt = r.UpdatedAt
	if r != expected {
		t.Fatalf("retry lost run context: %+v", r)
	}
}

func TestCreateUsesPerRunAgentFactoryAndContext(t *testing.T) {
	service := New(agent.NewMockRuntime(), nil)
	var command string
	service.SetAgentFactory(func(value string) contracts.Agent {
		command = value
		return agent.NewMockRuntime()
	})
	run, err := service.Create(context.Background(), contracts.SpawnRequest{RunID: "acp-run", WorkspaceID: "workspace", ThreadID: "thread", ChannelID: "channel", AgentCommand: "codex-acp"})
	if err != nil {
		t.Fatal(err)
	}
	if run.ThreadID != "thread" || run.ChannelID != "channel" || run.AgentCommand != "codex-acp" || command != "codex-acp" {
		t.Fatalf("run=%+v command=%q", run, command)
	}
}

func TestOutputEventsHaveUniqueIDsForRepeatedKinds(t *testing.T) {
	publisher := &eventPublisher{}
	service := New(nil, publisher)
	run, err := service.Create(context.Background(), contracts.SpawnRequest{RunID: "output-run", WorkspaceID: "workspace"})
	if err != nil {
		t.Fatal(err)
	}
	service.publishOutput(context.Background(), run, contracts.AgentOutput{RunID: run.ID, Kind: "output", Text: "one"})
	service.publishOutput(context.Background(), run, contracts.AgentOutput{RunID: run.ID, Kind: "output", Text: "two"})
	if len(publisher.events) != 3 || publisher.events[1].ID == publisher.events[2].ID {
		t.Fatalf("event IDs=%q,%q", publisher.events[1].ID, publisher.events[2].ID)
	}
}

func TestStartConsumesAgentOutputAndFinishes(t *testing.T) {
	publisher := &eventPublisher{}
	service := New(agent.NewMockRuntime(), publisher)
	if _, err := service.Create(context.Background(), contracts.SpawnRequest{RunID: "run-output", WorkspaceID: "workspace", Prompt: "task"}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Start(context.Background(), "run-output"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		run, err := service.Get(context.Background(), "run-output")
		if err == nil && run.Status == contracts.RunSucceeded {
			break
		}
		time.Sleep(time.Millisecond)
	}
	run, err := service.Get(context.Background(), "run-output")
	if err != nil || run.Status != contracts.RunSucceeded {
		t.Fatalf("run=%+v err=%v", run, err)
	}
	if len(publisher.events) < 3 {
		t.Fatalf("expected lifecycle and output events, got %d", len(publisher.events))
	}
}

func TestStartDetachesLifecycleFromCallerContext(t *testing.T) {
	publisher := &eventPublisher{}
	service := New(agent.NewMockRuntime(), publisher)
	if _, err := service.Create(context.Background(), contracts.SpawnRequest{RunID: "detached", WorkspaceID: "workspace", Prompt: "task"}); err != nil {
		t.Fatal(err)
	}
	caller, cancel := context.WithCancel(context.Background())
	if _, err := service.Start(caller, "detached"); err != nil {
		t.Fatal(err)
	}
	cancel()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if run, err := service.Get(context.Background(), "detached"); err == nil && run.Status == contracts.RunSucceeded {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if run, _ := service.Get(context.Background(), "detached"); run.Status != contracts.RunSucceeded {
		t.Fatalf("run did not finish after caller cancellation: %+v", run)
	}
	if len(publisher.events) < 4 {
		t.Fatalf("expected output and finished events, got %d", len(publisher.events))
	}
}
