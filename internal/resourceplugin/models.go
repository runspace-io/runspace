package resourceplugin

import (
	"context"
	"errors"
	"time"

	"github.com/runspace/runspace/internal/resourcegraph"
)

var (
	ErrInvalid  = errors.New("invalid resource connection")
	ErrNotFound = errors.New("resource connection not found")
)

type Capability struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Mode        string `json:"mode"`
	Risk        string `json:"risk"`
}

type AuthMethod struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	SecretLabel string `json:"secret_label"`
	Placeholder string `json:"placeholder"`
}

type Manifest struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Description  string       `json:"description"`
	ResourceType string       `json:"resource_type"`
	Placements   []string     `json:"placements"`
	AuthMethods  []AuthMethod `json:"auth_methods"`
	Capabilities []Capability `json:"capabilities"`
}

type Connection struct {
	ID           string         `json:"id"`
	WorkspaceID  string         `json:"workspace_id"`
	PluginID     string         `json:"plugin_id"`
	Title        string         `json:"title"`
	Placement    string         `json:"placement"`
	AuthMethod   string         `json:"auth_method"`
	AccessMode   string         `json:"access_mode"`
	OwnerID      string         `json:"owner_id"`
	Config       map[string]any `json:"config,omitempty"`
	Secret       []byte         `json:"-"`
	Capabilities []Capability   `json:"capabilities"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

type ConnectRequest struct {
	PluginID   string         `json:"plugin_id"`
	Title      string         `json:"title"`
	Placement  string         `json:"placement"`
	AuthMethod string         `json:"auth_method"`
	AccessMode string         `json:"access_mode"`
	Credential string         `json:"credential"`
	Config     map[string]any `json:"config,omitempty"`
}

type Availability struct {
	ResourceID string    `json:"resource_id"`
	Status     string    `json:"status"`
	Reason     string    `json:"reason,omitempty"`
	CheckedAt  time.Time `json:"checked_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}

type Store interface {
	SaveResourceConnection(context.Context, Connection) error
	ListResourceConnections(context.Context, string) ([]Connection, error)
	GetResourceConnection(context.Context, string) (Connection, error)
}

type Authorizer interface {
	CanRead(context.Context, string, string) error
	CanWrite(context.Context, string, string) error
}

type Graph interface {
	UpsertNode(context.Context, string, resourcegraph.Node) (resourcegraph.Node, error)
}
