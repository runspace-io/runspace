package hostagent

import "time"

type CapabilityDescriptor struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Mode        string `json:"mode"`
	Risk        string `json:"risk"`
}

type AdapterManifest struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	Executable   string                 `json:"executable"`
	ResourceType string                 `json:"resource_type"`
	Capabilities []CapabilityDescriptor `json:"capabilities"`
}

type AdapterDiscovery struct {
	Manifest AdapterManifest `json:"manifest"`
	Status   string          `json:"status"`
	Path     string          `json:"path,omitempty"`
}

type LocalCapabilityResource struct {
	ID           string                 `json:"id"`
	AdapterID    string                 `json:"adapter_id"`
	Title        string                 `json:"title"`
	Profile      string                 `json:"profile,omitempty"`
	GatewayURL   string                 `json:"gateway_url"`
	WorkspaceID  string                 `json:"workspace_id"`
	Capabilities []CapabilityDescriptor `json:"capabilities"`
	CreatedAt    time.Time              `json:"created_at"`
}

type CapabilityQueryRequest struct {
	Capability string `json:"capability"`
	Query      string `json:"query"`
	Limit      int    `json:"limit"`
}

type CapabilityQueryResult struct {
	ResourceID string            `json:"resource_id"`
	Capability string            `json:"capability"`
	Matches    []CapabilityMatch `json:"matches"`
	Truncated  bool              `json:"truncated"`
}

type CapabilityMatch struct {
	Title     string         `json:"title"`
	Summary   string         `json:"summary,omitempty"`
	Reference string         `json:"reference,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}
