package workspace

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type MemoryService struct {
	mu           sync.RWMutex
	clock        Clock
	seq          uint64
	workspaces   map[string]Workspace
	members      map[string]map[string]Member
	repositories map[string]Repository
	store        Store
	graph        GraphProjector
}

func (s *MemoryService) SetStore(store Store) { s.mu.Lock(); defer s.mu.Unlock(); s.store = store }

var _ Service = (*MemoryService)(nil)

func NewMemoryService(clock Clock) *MemoryService {
	if clock == nil {
		clock = time.Now
	}
	return &MemoryService{clock: clock, workspaces: make(map[string]Workspace), members: make(map[string]map[string]Member), repositories: make(map[string]Repository)}
}
func (s *MemoryService) nextID(prefix string) string {
	s.seq++
	return fmt.Sprintf("%s_%d", prefix, s.seq)
}
func requireUser(userID string) error {
	if strings.TrimSpace(userID) == "" {
		return ErrUnauthorized
	}
	return nil
}
func (s *MemoryService) roleLocked(workspaceID, userID string) (Role, error) {
	m, ok := s.members[workspaceID][userID]
	if !ok {
		return "", ErrForbidden
	}
	return m.Role, nil
}
func (s *MemoryService) authorizeLocked(workspaceID, userID string, action Action) error {
	role, err := s.roleLocked(workspaceID, userID)
	if err != nil {
		return err
	}
	if !Allows(role, action) {
		return ErrForbidden
	}
	return nil
}

func (s *MemoryService) CreateWorkspace(ctx context.Context, userID string, req CreateWorkspaceRequest) (Workspace, error) {
	if err := ctx.Err(); err != nil {
		return Workspace{}, err
	}
	if err := requireUser(userID); err != nil {
		return Workspace{}, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return Workspace{}, ErrInvalidInput
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clock().UTC()
	for _, existing := range s.workspaces {
		if existing.Slug == slugify(name) {
			return Workspace{}, ErrAlreadyExists
		}
	}
	id := s.nextID("ws")
	ws := Workspace{ID: id, Slug: slugify(name), Name: name, CreatedBy: userID, CreatedAt: now, UpdatedAt: now}
	s.workspaces[id] = ws
	s.members[id] = map[string]Member{userID: {WorkspaceID: id, UserID: userID, Role: RoleOwner, CreatedAt: now}}
	if s.store != nil {
		if err := s.store.CreateWorkspaceWithMember(ctx, ws, s.members[id][userID]); err != nil {
			return Workspace{}, err
		}
	}
	return ws, nil
}
func (s *MemoryService) ListWorkspaces(ctx context.Context, userID string) ([]Workspace, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := requireUser(userID); err != nil {
		return nil, err
	}
	s.mu.RLock()
	store := s.store
	if store != nil {
		s.mu.RUnlock()
		if persisted, err := store.ListWorkspaces(ctx, userID); err == nil && persisted != nil {
			for _, item := range persisted {
				s.hydrate(userID, item)
			}
			return persisted, nil
		}
		s.mu.RLock()
		defer s.mu.RUnlock()
	} else {
		defer s.mu.RUnlock()
	}
	out := make([]Workspace, 0)
	for id, members := range s.members {
		if _, ok := members[userID]; ok {
			out = append(out, s.workspaces[id])
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}
func (s *MemoryService) GetWorkspace(ctx context.Context, userID, id string) (Workspace, error) {
	if err := ctx.Err(); err != nil {
		return Workspace{}, err
	}
	if err := requireUser(userID); err != nil {
		return Workspace{}, err
	}
	s.mu.RLock()
	if _, ok := s.workspaces[id]; !ok {
		store := s.store
		s.mu.RUnlock()
		if store == nil {
			return Workspace{}, ErrNotFound
		}
		items, err := store.ListWorkspaces(ctx, userID)
		if err != nil {
			return Workspace{}, err
		}
		for _, item := range items {
			if item.ID == id {
				s.hydrate(userID, item)
				return item, nil
			}
		}
		return Workspace{}, ErrNotFound
	}
	if _, ok := s.members[id][userID]; !ok {
		s.mu.RUnlock()
		return Workspace{}, ErrForbidden
	}
	item := s.workspaces[id]
	s.mu.RUnlock()
	return item, nil
}

func (s *MemoryService) hydrate(userID string, item Workspace) {
	s.mu.Lock()
	if parts := strings.SplitN(item.ID, "_", 2); len(parts) == 2 {
		if value, err := strconv.ParseUint(parts[1], 10, 64); err == nil && value > s.seq {
			s.seq = value
		}
	}
	s.workspaces[item.ID] = item
	if s.members[item.ID] == nil {
		s.members[item.ID] = make(map[string]Member)
	}
	role := RoleMember
	if item.CreatedBy == userID {
		role = RoleOwner
	}
	s.members[item.ID][userID] = Member{WorkspaceID: item.ID, UserID: userID, Role: role, CreatedAt: item.CreatedAt}
	s.mu.Unlock()
}
func (s *MemoryService) ConnectResource(ctx context.Context, userID, workspaceID string, req ConnectResourceRequest) (Resource, error) {
	if err := ctx.Err(); err != nil {
		return Repository{}, err
	}
	if err := requireUser(userID); err != nil {
		return Repository{}, err
	}
	req = normalizedRepositoryRequest(req)
	if !validRepositoryRequest(req) {
		return Repository{}, ErrInvalidInput
	}
	if err := s.ensureRepositoryWorkspace(ctx, userID, workspaceID); err != nil {
		return Repository{}, err
	}
	s.mu.RLock()
	hasStore := s.store != nil
	s.mu.RUnlock()
	if hasStore {
		if _, err := s.ListRepositories(ctx, userID, workspaceID); err != nil {
			return Repository{}, err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.workspaces[workspaceID]; !ok {
		return Repository{}, ErrNotFound
	}
	if err := s.authorizeLocked(workspaceID, userID, ActionConnectResource); err != nil {
		return Repository{}, err
	}
	if s.repositoryDuplicateLocked(workspaceID, req) {
		return Repository{}, ErrAlreadyExists
	}
	now := s.clock().UTC()
	s.seq++
	repositoryID := fmt.Sprintf("repo_%d_%d", now.UnixNano(), s.seq)
	r := Repository{ID: repositoryID, WorkspaceID: workspaceID, Provider: req.Provider, FullName: req.FullName, CloneURL: req.CloneURL, DefaultBranch: req.DefaultBranch, CreatedBy: userID, CreatedAt: now}
	s.repositories[r.ID] = r
	if s.store != nil {
		if err := s.store.CreateRepository(ctx, r); err != nil {
			return Repository{}, err
		}
	}
	s.projectResource(ctx, r)
	return r, nil
}

func (s *MemoryService) ConnectRepository(ctx context.Context, userID, workspaceID string, req ConnectRepositoryRequest) (Repository, error) {
	return s.ConnectResource(ctx, userID, workspaceID, req)
}

func (s *MemoryService) ensureRepositoryWorkspace(ctx context.Context, userID, workspaceID string) error {
	_, err := s.GetWorkspace(ctx, userID, workspaceID)
	return err
}

func (s *MemoryService) repositoryDuplicateLocked(workspaceID string, req ConnectRepositoryRequest) bool {
	for _, item := range s.repositories {
		if item.WorkspaceID != workspaceID {
			continue
		}
		if req.Provider == "mirror" || req.Provider == "folder" {
			if item.CloneURL == req.CloneURL {
				return true
			}
			continue
		}
		if item.Provider == req.Provider && item.FullName == req.FullName || item.CloneURL == req.CloneURL {
			return true
		}
	}
	return false
}

func normalizedRepositoryRequest(req ConnectRepositoryRequest) ConnectRepositoryRequest {
	req.Provider = strings.TrimSpace(req.Provider)
	req.FullName = strings.TrimSpace(req.FullName)
	req.CloneURL = strings.TrimSpace(req.CloneURL)
	req.DefaultBranch = strings.TrimSpace(req.DefaultBranch)
	return req
}

func validRepositoryRequest(req ConnectRepositoryRequest) bool {
	return req.Provider != "" &&
		req.FullName != "" &&
		req.CloneURL != "" &&
		(req.DefaultBranch != "" || req.Provider == "folder")
}
func (s *MemoryService) ListResources(ctx context.Context, userID, workspaceID string) ([]Resource, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := requireUser(userID); err != nil {
		return nil, err
	}
	s.mu.RLock()
	store := s.store
	s.mu.RUnlock()
	if store != nil {
		if persisted, err := store.ListRepositories(ctx, userID, workspaceID); err == nil && persisted != nil {
			s.hydrateRepositories(persisted)
			return persisted, nil
		}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.workspaces[workspaceID]; !ok {
		return nil, ErrNotFound
	}
	if _, ok := s.members[workspaceID][userID]; !ok {
		return nil, ErrForbidden
	}
	out := make([]Repository, 0)
	for _, r := range s.repositories {
		if r.WorkspaceID == workspaceID {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (s *MemoryService) ListRepositories(ctx context.Context, userID, workspaceID string) ([]Repository, error) {
	return s.ListResources(ctx, userID, workspaceID)
}
