package workspace

import "time"

// Role controls access to workspace resources. Roles are ordered from most to
// least privileged by the authorization policy in policy.go.
type Role string

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
	RoleViewer Role = "viewer"
)

type Workspace struct {
	ID        string    `json:"id"`
	Slug      string    `json:"slug"`
	Name      string    `json:"name"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Member struct {
	WorkspaceID string    `json:"workspace_id"`
	UserID      string    `json:"user_id"`
	Role        Role      `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
}

// Resource is a workspace-mounted source of files and tools. Providers may be
// remote Git repositories, local Git mirrors, or plain host folders.
type Resource struct {
	ID            string    `json:"id"`
	WorkspaceID   string    `json:"workspace_id"`
	Provider      string    `json:"provider"`
	FullName      string    `json:"full_name"`
	CloneURL      string    `json:"clone_url"`
	DefaultBranch string    `json:"default_branch"`
	CreatedBy     string    `json:"created_by"`
	CreatedAt     time.Time `json:"created_at"`
}

// Repository is kept as a source-compatible alias while repository-specific
// services migrate to the broader Resource vocabulary.
type Repository = Resource
