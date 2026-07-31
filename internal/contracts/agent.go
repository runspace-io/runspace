package contracts

import "context"

// Agent is the stable boundary for an executable agent runtime.
// Implementations may wrap OpenCode, OpenHands, Claude Code, or Codex CLI.
type Agent interface {
	Spawn(context.Context, SpawnRequest) (AgentRun, error)
	Run(context.Context, RunRequest) (AgentRun, error)
	Stop(context.Context, StopRequest) error
	Stream(context.Context, StreamRequest) (<-chan AgentOutput, error)
}

type InputAgent interface {
	Send(context.Context, InputRequest) error
}

type SpawnRequest struct {
	RunID            string
	WorkspaceID      string
	ThreadID         string
	ChannelID        string
	Repository       string
	Prompt           string
	AgentCommand     string
	WorkingDirectory string
}

type RunRequest struct {
	RunID string
}

type StopRequest struct {
	RunID string
}

type StreamRequest struct {
	RunID string
}

type InputRequest struct {
	RunID string
	Text  string
}

type AgentRun struct {
	ID     string
	Status RunState
}

type AgentOutput struct {
	RunID string
	Kind  string
	Text  string
}
