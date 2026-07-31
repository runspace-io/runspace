package contracts

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type fakeAgent struct{}

var _ Agent = fakeAgent{}

func (fakeAgent) Spawn(context.Context, SpawnRequest) (AgentRun, error) {
	return AgentRun{ID: "r", Status: RunQueued}, nil
}
func (fakeAgent) Run(context.Context, RunRequest) (AgentRun, error) {
	return AgentRun{ID: "r", Status: RunRunning}, nil
}
func (fakeAgent) Stop(context.Context, StopRequest) error { return nil }
func (fakeAgent) Stream(context.Context, StreamRequest) (<-chan AgentOutput, error) {
	return make(chan AgentOutput), nil
}

type fakeGit struct{}

var _ GitProvider = fakeGit{}

func (fakeGit) Clone(context.Context, CloneRequest) (CloneResult, error) { return CloneResult{}, nil }
func (fakeGit) CreateBranch(context.Context, BranchRequest) (BranchResult, error) {
	return BranchResult{}, nil
}
func (fakeGit) OpenPR(context.Context, PullRequest) (PullRequestResult, error) {
	return PullRequestResult{}, nil
}
func (fakeGit) Merge(context.Context, MergeRequest) error     { return nil }
func (fakeGit) Comment(context.Context, CommentRequest) error { return nil }

func TestEventEnvelope(t *testing.T) {
	e, err := NewEvent("e1", "agent.spawn", "w1", "u1", "user", map[string]string{"run_id": "r1"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Validate(); err != nil {
		t.Fatal(err)
	}
	if !json.Valid(e.Payload) {
		t.Fatal("payload is not JSON")
	}
	e.ID = ""
	if !errors.Is(e.Validate(), ErrInvalidEvent) {
		t.Fatal("expected invalid event")
	}
}

func TestRunTransitions(t *testing.T) {
	valid := [][2]RunState{{RunQueued, RunRunning}, {RunRunning, RunSucceeded}, {RunRunning, RunStopping}, {RunStopping, RunCancelled}}
	for _, pair := range valid {
		if err := TransitionRun(pair[0], pair[1]); err != nil {
			t.Errorf("valid transition: %v", err)
		}
	}
	if err := TransitionRun(RunSucceeded, RunRunning); err == nil {
		t.Fatal("expected terminal transition failure")
	}
	if err := TransitionRun(RunQueued, RunQueued); err != nil {
		t.Fatal(err)
	}
}
