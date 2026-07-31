package repository

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/runspace/runspace/internal/contracts"
	"github.com/runspace/runspace/internal/sandbox"
	"github.com/runspace/runspace/internal/workspace"
)

type Cloner interface {
	Clone(context.Context, contracts.CloneRequest) (contracts.CloneResult, error)
}

type Service struct {
	workspaces workspace.Service
	resolver   sandbox.RootResolver
	git        Cloner
	publisher  StatusPublisher
	mu         sync.Mutex
	watchers   map[string]*Watcher
}

func NewService(workspaces workspace.Service, resolver sandbox.RootResolver, cloner Cloner) (*Service, error) {
	return NewServiceWithPublisher(workspaces, resolver, cloner, nil)
}

func NewServiceWithPublisher(workspaces workspace.Service, resolver sandbox.RootResolver, cloner Cloner, publisher StatusPublisher) (*Service, error) {
	if workspaces == nil || resolver == nil || cloner == nil {
		return nil, errors.New("repository dependencies are required")
	}
	return &Service{workspaces: workspaces, resolver: resolver, git: cloner, publisher: publisher, watchers: make(map[string]*Watcher)}, nil
}

func (s *Service) Clone(ctx context.Context, userID, workspaceID, repositoryID string) (contracts.CloneResult, error) {
	repositories, err := s.workspaces.ListRepositories(ctx, userID, workspaceID)
	if err != nil {
		return contracts.CloneResult{}, err
	}
	selected := connectedRepository(repositories, repositoryID)
	if selected.ID == "" {
		return contracts.CloneResult{}, workspace.ErrNotFound
	}
	destination, err := s.resolver.Root(ctx, workspaceID, repositoryID)
	if err != nil {
		return contracts.CloneResult{}, err
	}
	exists, err := prepareExistingCheckout(destination, selected)
	if err != nil {
		return contracts.CloneResult{}, err
	}
	if exists {
		s.startWatcher(ctx, workspaceID, repositoryID, destination)
		return contracts.CloneResult{Path: destination, Ref: selected.DefaultBranch}, nil
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o750); err != nil {
		return contracts.CloneResult{}, err
	}
	result, err := s.git.Clone(ctx, contracts.CloneRequest{RepositoryURL: selected.CloneURL, Destination: destination, Ref: selected.DefaultBranch})
	if err != nil {
		return contracts.CloneResult{}, err
	}
	if err := prepareAgentCheckout(destination); err != nil {
		return contracts.CloneResult{}, err
	}
	s.startWatcher(ctx, workspaceID, repositoryID, destination)
	return result, nil
}

func connectedRepository(
	repositories []workspace.Repository,
	repositoryID string,
) workspace.Repository {
	for _, repository := range repositories {
		if repository.ID == repositoryID {
			return repository
		}
	}
	return workspace.Repository{}
}

func prepareExistingCheckout(destination string, repository workspace.Repository) (bool, error) {
	if _, err := os.Stat(destination); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	placeholder, err := isUnseededRemotePlaceholder(destination, repository)
	if err != nil {
		return false, err
	}
	if placeholder {
		if err := os.RemoveAll(filepath.Join(destination, ".git")); err != nil {
			return false, err
		}
		if err := os.Remove(destination); err != nil {
			return false, err
		}
		return false, nil
	}
	return true, prepareAgentCheckout(destination)
}

func isUnseededRemotePlaceholder(
	destination string,
	repository workspace.Repository,
) (bool, error) {
	if repository.Provider == "folder" || strings.HasPrefix(repository.CloneURL, "local-mirror://") {
		return false, nil
	}
	entries, err := os.ReadDir(destination)
	if err != nil {
		return false, err
	}
	if len(entries) != 1 || entries[0].Name() != ".git" || !entries[0].IsDir() {
		return false, nil
	}
	return true, nil
}

func (s *Service) startWatcher(ctx context.Context, workspaceID, repositoryID, path string) {
	status, ok := s.git.(StatusReader)
	if !ok || s.publisher == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.watchers[watcherKey(workspaceID, repositoryID)]; exists {
		return
	}
	watcher, err := NewWatcher(path, workspaceID, repositoryID, status, s.publisher)
	if err != nil {
		return
	}
	if watcher.Start(context.WithoutCancel(ctx)) != nil {
		return
	}
	s.watchers[watcherKey(workspaceID, repositoryID)] = watcher
}

func watcherKey(workspaceID, repositoryID string) string {
	return strings.TrimSpace(workspaceID) + ":" + strings.TrimSpace(repositoryID)
}

func (s *Service) Close() {
	s.mu.Lock()
	watchers := make([]*Watcher, 0, len(s.watchers))
	for _, watcher := range s.watchers {
		watchers = append(watchers, watcher)
	}
	s.watchers = make(map[string]*Watcher)
	s.mu.Unlock()
	for _, watcher := range watchers {
		watcher.Stop()
	}
}
