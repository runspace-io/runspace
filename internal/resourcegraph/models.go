package resourcegraph

import "time"

type Kind string

const (
	KindResource   Kind = "resource"
	KindTask       Kind = "task"
	KindArtifact   Kind = "artifact"
	KindAction     Kind = "action"
	KindDiscussion Kind = "discussion"
	KindIdentity   Kind = "identity"
	KindPolicy     Kind = "policy"
	KindEvent      Kind = "event"
)

type Node struct {
	ID          string         `json:"id"`
	WorkspaceID string         `json:"workspace_id"`
	Kind        Kind           `json:"kind"`
	Type        string         `json:"type"`
	Title       string         `json:"title"`
	Summary     string         `json:"summary,omitempty"`
	ExternalRef string         `json:"external_ref,omitempty"`
	OwnerID     string         `json:"owner_id"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type Edge struct {
	ID          string         `json:"id"`
	WorkspaceID string         `json:"workspace_id"`
	FromID      string         `json:"from_id"`
	ToID        string         `json:"to_id"`
	Relation    string         `json:"relation"`
	CreatedBy   string         `json:"created_by"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

type Query struct {
	Kind     Kind
	Type     string
	Text     string
	ThreadID string
	Limit    int
}

type Context struct {
	Node     Node   `json:"node"`
	Incoming []Edge `json:"incoming"`
	Outgoing []Edge `json:"outgoing"`
}
