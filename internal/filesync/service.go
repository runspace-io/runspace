package filesync

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/runspace/runspace/internal/sandbox"
	"github.com/runspace/runspace/internal/workspace"
)

var (
	ErrInvalidInput = errors.New("invalid sync session input")
	ErrNotFound     = errors.New("sync session not found")
)

type WorkspaceService interface {
	CanWrite(context.Context, string, string) error
	ListRepositories(context.Context, string, string) ([]workspace.Repository, error)
}

type RegisterRequest struct {
	UserID       string   `json:"-"`
	WorkspaceID  string   `json:"workspace_id"`
	RepositoryID string   `json:"repository_id"`
	DeviceID     string   `json:"device_id"`
	DeviceName   string   `json:"device_name"`
	Addresses    []string `json:"addresses"`
	Branch       string   `json:"branch"`
	Git          bool     `json:"git"`
}

type Session struct {
	ID               string       `json:"id"`
	WorkspaceID      string       `json:"workspace_id"`
	RepositoryID     string       `json:"repository_id"`
	FolderID         string       `json:"folder_id"`
	HostDeviceID     string       `json:"host_device_id"`
	GatewayDeviceID  string       `json:"gateway_device_id"`
	GatewayAddresses []string     `json:"gateway_addresses"`
	Status           FolderStatus `json:"status"`
	CreatedAt        time.Time    `json:"created_at"`
}

type Service struct {
	workspaces       WorkspaceService
	resolver         sandbox.RootResolver
	engine           Engine
	gatewayAddresses []string
	mu               sync.RWMutex
	sessions         map[string]Session
}

func NewService(
	workspaces WorkspaceService,
	resolver sandbox.RootResolver,
	engine Engine,
	gatewayAddresses []string,
) (*Service, error) {
	if workspaces == nil || resolver == nil || engine == nil {
		return nil, errors.New("sync service dependencies are required")
	}
	return &Service{
		workspaces:       workspaces,
		resolver:         resolver,
		engine:           engine,
		gatewayAddresses: compact(gatewayAddresses),
		sessions:         make(map[string]Session),
	}, nil
}

func (s *Service) Register(ctx context.Context, request RegisterRequest) (Session, error) {
	if !validRegisterRequest(request) {
		return Session{}, ErrInvalidInput
	}
	if err := s.workspaces.CanWrite(ctx, request.WorkspaceID, request.UserID); err != nil {
		return Session{}, err
	}
	repositories, err := s.workspaces.ListRepositories(ctx, request.UserID, request.WorkspaceID)
	if err != nil {
		return Session{}, err
	}
	if !isMirrorRepository(repositories, request.RepositoryID) {
		return Session{}, ErrNotFound
	}
	root, err := s.resolver.Root(ctx, request.WorkspaceID, request.RepositoryID)
	if err != nil {
		return Session{}, err
	}
	if err := os.MkdirAll(root, 0o750); err != nil {
		return Session{}, err
	}
	// The repository volume is shared with the Syncthing and agent containers,
	// which intentionally run as an unprivileged UID. The gateway may have
	// created this directory under a different UID (notably in development), so
	// ownership alone cannot grant the sync process access.
	if err := os.Chmod(root, 0o777); err != nil {
		return Session{}, err
	}
	if request.Git {
		if err := initializeRepository(ctx, root, request.Branch); err != nil {
			return Session{}, err
		}
	}
	gatewayID, err := s.engine.DeviceID(ctx)
	if err != nil {
		return Session{}, err
	}
	folderID := folderID(request.WorkspaceID, request.RepositoryID)
	if err := s.engine.UpsertDevice(ctx, Device{
		ID:        request.DeviceID,
		Name:      request.DeviceName,
		Addresses: compact(request.Addresses),
	}); err != nil {
		return Session{}, err
	}
	if err := s.engine.UpsertFolder(ctx, Folder{
		ID:               folderID,
		Label:            request.RepositoryID,
		Path:             root,
		Type:             "sendreceive",
		FSWatcherEnabled: true,
		FSWatcherDelayS:  1,
		IgnorePerms:      true,
		Devices:          []FolderDevice{{DeviceID: request.DeviceID}},
	}); err != nil {
		return Session{}, err
	}
	if err := s.engine.SetIgnores(ctx, folderID, MirrorIgnorePatterns()); err != nil {
		return Session{}, err
	}
	status, err := s.engine.Status(ctx, folderID)
	if err != nil {
		status = FolderStatus{State: "starting", Error: err.Error()}
	}
	session := Session{
		ID:               sessionKey(request.WorkspaceID, request.RepositoryID),
		WorkspaceID:      request.WorkspaceID,
		RepositoryID:     request.RepositoryID,
		FolderID:         folderID,
		HostDeviceID:     request.DeviceID,
		GatewayDeviceID:  gatewayID,
		GatewayAddresses: append([]string(nil), s.gatewayAddresses...),
		Status:           status,
		CreatedAt:        time.Now().UTC(),
	}
	s.mu.Lock()
	s.sessions[session.ID] = session
	s.mu.Unlock()
	return session, nil
}

func initializeRepository(ctx context.Context, root, branch string) error {
	if _, err := os.Stat(root + string(os.PathSeparator) + ".git"); err == nil {
		return nil
	}
	branch = strings.TrimSpace(branch)
	if branch == "" {
		branch = "main"
	}
	if !validBranchName(branch) {
		return ErrInvalidInput
	}
	output, err := exec.CommandContext(ctx, "git", "init", "--initial-branch="+branch, root).CombinedOutput()
	if err != nil {
		return errors.New("initialize mirror repository: " + strings.TrimSpace(string(output)))
	}
	return nil
}

func validBranchName(branch string) bool {
	if strings.HasPrefix(branch, "-") ||
		strings.ContainsAny(branch, " ~^:?*[\\") ||
		strings.Contains(branch, "..") ||
		strings.HasSuffix(branch, ".") ||
		strings.HasSuffix(branch, "/") {
		return false
	}
	for _, character := range branch {
		if character < 32 || character == 127 {
			return false
		}
	}
	return true
}

func (s *Service) Status(ctx context.Context, userID, workspaceID, repositoryID string) (Session, error) {
	if err := s.workspaces.CanWrite(ctx, workspaceID, userID); err != nil {
		return Session{}, err
	}
	key := sessionKey(workspaceID, repositoryID)
	s.mu.RLock()
	session, exists := s.sessions[key]
	s.mu.RUnlock()
	if !exists {
		repositories, err := s.workspaces.ListRepositories(ctx, userID, workspaceID)
		if err != nil {
			return Session{}, err
		}
		if !isMirrorRepository(repositories, repositoryID) {
			return Session{}, ErrNotFound
		}
		gatewayID, err := s.engine.DeviceID(ctx)
		if err != nil {
			return Session{}, err
		}
		session = Session{
			ID:               key,
			WorkspaceID:      workspaceID,
			RepositoryID:     repositoryID,
			FolderID:         folderID(workspaceID, repositoryID),
			GatewayDeviceID:  gatewayID,
			GatewayAddresses: append([]string(nil), s.gatewayAddresses...),
		}
	}
	status, err := s.engine.Status(ctx, session.FolderID)
	if err != nil {
		status = FolderStatus{State: "error", Error: err.Error()}
	}
	session.Status = status
	s.mu.Lock()
	s.sessions[key] = session
	s.mu.Unlock()
	return session, nil
}

func isMirrorRepository(repositories []workspace.Repository, repositoryID string) bool {
	for _, repository := range repositories {
		if repository.ID == repositoryID &&
			(repository.Provider == "mirror" || repository.Provider == "folder") {
			return true
		}
	}
	return false
}

func validRegisterRequest(request RegisterRequest) bool {
	return strings.TrimSpace(request.UserID) != "" &&
		strings.TrimSpace(request.WorkspaceID) != "" &&
		strings.TrimSpace(request.RepositoryID) != "" &&
		strings.TrimSpace(request.DeviceID) != "" &&
		len(compact(request.Addresses)) > 0
}

func compact(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, value)
		}
	}
	return result
}

func sessionKey(workspaceID, repositoryID string) string {
	return strings.TrimSpace(workspaceID) + ":" + strings.TrimSpace(repositoryID)
}

func folderID(workspaceID, repositoryID string) string {
	return "runspace-" + strings.TrimSpace(workspaceID) + "-" + strings.TrimSpace(repositoryID)
}

func MirrorIgnorePatterns() []string {
	names := []string{".git", "node_modules", ".pnpm-store", ".next", ".cache", "test-results"}
	patterns := make([]string, 0, len(names)*4)
	for _, name := range names {
		patterns = append(
			patterns,
			"(?d)"+name,
			"(?d)"+name+"/**",
			"(?d)**/"+name,
			"(?d)**/"+name+"/**",
		)
	}
	return patterns
}
