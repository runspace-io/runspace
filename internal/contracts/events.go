package contracts

import (
	"bytes"
	"encoding/json"
	"errors"
	"time"
)

var ErrInvalidEvent = errors.New("invalid event envelope")

// Stable event names shared by agent and Git orchestration publishers.
const (
	EventAgentSpawned     = "agent.spawned"
	EventAgentOutput      = "agent.output"
	EventAgentFinished    = "agent.finished"
	EventAgentStopped     = "agent.stopped"
	EventGitBranchCreated = "git.branch.created"
	EventGitCommit        = "git.commit"
	EventGitStatusChanged = "git.status.changed"
)

// Agent task events carry host-executed chat turns onto the workspace bus so
// every member with a grant sees the same transcript as the owner.
const (
	EventAgentTaskMessage      = "agent.task.message"
	EventAgentTaskStatus       = "agent.task.status"
	EventAgentQuestionAsked    = "agent.question.asked"
	EventAgentQuestionAnswered = "agent.question.answered"
)

// EventEnvelope is the versioned, append-only contract carried by the bus.
type EventEnvelope struct {
	ID           string          `json:"id"`
	Type         string          `json:"type"`
	Version      int             `json:"version"`
	WorkspaceID  string          `json:"workspace_id"`
	RepositoryID string          `json:"repository_id,omitempty"`
	ChannelID    string          `json:"channel_id,omitempty"`
	ActorID      string          `json:"actor_id"`
	ActorType    string          `json:"actor_type"`
	OccurredAt   time.Time       `json:"occurred_at"`
	Payload      json.RawMessage `json:"payload"`
}

func NewEvent(id, eventType, workspaceID, actorID, actorType string, payload any, now time.Time) (EventEnvelope, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return EventEnvelope{}, err
	}
	e := EventEnvelope{ID: id, Type: eventType, Version: 1, WorkspaceID: workspaceID, ActorID: actorID, ActorType: actorType, OccurredAt: now.UTC(), Payload: b}
	return e, e.Validate()
}

func (e EventEnvelope) Validate() error {
	if !hasRequiredFields(e) {
		return ErrInvalidEvent
	}
	payload := bytes.TrimSpace(e.Payload)
	if len(payload) == 0 || bytes.Equal(payload, []byte("null")) || !json.Valid(payload) {
		return ErrInvalidEvent
	}
	return nil
}

func hasRequiredFields(event EventEnvelope) bool {
	return event.ID != "" && event.Type != "" && event.Version >= 1 && event.WorkspaceID != "" && event.ActorID != "" && event.ActorType != "" && !event.OccurredAt.IsZero()
}
