package workspace

import (
	"context"
	"time"
)

type CreateWorkspaceRequest struct {
	Name string `json:"name"`
}
type ConnectResourceRequest struct {
	Provider      string `json:"provider"`
	FullName      string `json:"full_name"`
	CloneURL      string `json:"clone_url"`
	DefaultBranch string `json:"default_branch"`
}
type ConnectRepositoryRequest = ConnectResourceRequest

type Service interface {
	CreateWorkspace(context.Context, string, CreateWorkspaceRequest) (Workspace, error)
	ListWorkspaces(context.Context, string) ([]Workspace, error)
	GetWorkspace(context.Context, string, string) (Workspace, error)
	ConnectResource(context.Context, string, string, ConnectResourceRequest) (Resource, error)
	ListResources(context.Context, string, string) ([]Resource, error)
	// Deprecated compatibility methods for repository-specific consumers.
	ConnectRepository(context.Context, string, string, ConnectRepositoryRequest) (Repository, error)
	ListRepositories(context.Context, string, string) ([]Repository, error)
	AddMember(context.Context, string, string, string, Role) (Member, error)
	ListMembers(context.Context, string, string) ([]Member, error)
}

// Store is the durable workspace boundary. Implementations may be backed by
// PostgreSQL; the in-memory service remains useful for isolated tests.
type Store interface {
	CreateWorkspaceWithMember(context.Context, Workspace, Member) error
	ListWorkspaces(context.Context, string) ([]Workspace, error)
	CreateRepository(context.Context, Repository) error
	ListRepositories(context.Context, string, string) ([]Repository, error)
}

type MembershipStore interface {
	CanRead(context.Context, string, string) error
	CanWrite(context.Context, string, string) error
}

type MemberListStore interface {
	ListMembers(context.Context, string, string) ([]Member, error)
}

type MemberStore interface {
	CreateMember(context.Context, Member) error
}

type WorkspaceStore interface {
	GetWorkspace(context.Context, string, string) (Workspace, error)
}

type Clock func() time.Time
