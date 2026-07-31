package workspace

import (
	"context"
	"sort"
	"strings"
)

func (s *MemoryService) AddMember(ctx context.Context, userID, workspaceID, target string, role Role) (Member, error) {
	if err := ctx.Err(); err != nil {
		return Member{}, err
	}
	if err := requireUser(userID); err != nil {
		return Member{}, err
	}
	target = strings.TrimSpace(target)
	if target == "" || !role.valid() || role == RoleOwner {
		return Member{}, ErrInvalidInput
	}
	if err := s.hydrateWorkspaceMembership(ctx, userID, workspaceID); err != nil {
		return Member{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.workspaces[workspaceID]; !ok {
		return Member{}, ErrNotFound
	}
	if err := s.authorizeLocked(workspaceID, userID, ActionManageMembers); err != nil {
		return Member{}, err
	}
	if _, ok := s.members[workspaceID][target]; ok {
		return Member{}, ErrAlreadyExists
	}
	m := Member{WorkspaceID: workspaceID, UserID: target, Role: role, CreatedAt: s.clock().UTC()}
	s.members[workspaceID][target] = m
	store := s.store
	if err := persistMember(ctx, store, m); err != nil {
		return Member{}, err
	}
	return m, nil
}

func (s *MemoryService) hydrateWorkspaceMembership(
	ctx context.Context,
	userID string,
	workspaceID string,
) error {
	s.mu.RLock()
	_, exists := s.workspaces[workspaceID]
	store := s.store
	s.mu.RUnlock()
	if exists {
		return nil
	}
	workspaceStore, workspaceOK := store.(WorkspaceStore)
	memberStore, membersOK := store.(MemberListStore)
	if !workspaceOK || !membersOK {
		return ErrNotFound
	}
	item, err := workspaceStore.GetWorkspace(ctx, userID, workspaceID)
	if err != nil {
		return err
	}
	members, err := memberStore.ListMembers(ctx, userID, workspaceID)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, loaded := s.workspaces[workspaceID]; loaded {
		return nil
	}
	s.workspaces[workspaceID] = item
	s.members[workspaceID] = make(map[string]Member, len(members))
	for _, member := range members {
		s.members[workspaceID][member.UserID] = member
	}
	return nil
}

func persistMember(ctx context.Context, store Store, member Member) error {
	memberStore, ok := store.(MemberStore)
	if !ok {
		return nil
	}
	return memberStore.CreateMember(ctx, member)
}

func (s *MemoryService) ListMembers(ctx context.Context, userID, workspaceID string) ([]Member, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := requireUser(userID); err != nil {
		return nil, err
	}
	if err := s.CanRead(ctx, workspaceID, userID); err != nil {
		return nil, err
	}
	s.mu.RLock()
	store := s.store
	members := append([]Member(nil), memberValues(s.members[workspaceID])...)
	s.mu.RUnlock()
	if memberStore, ok := store.(MemberListStore); ok {
		if persisted, err := memberStore.ListMembers(ctx, userID, workspaceID); err == nil && persisted != nil {
			return persisted, nil
		}
	}
	sortMembers(members)
	return members, nil
}

func memberValues(items map[string]Member) []Member {
	result := make([]Member, 0, len(items))
	for _, item := range items {
		result = append(result, item)
	}
	return result
}

func sortMembers(items []Member) {
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
}

func (s *MemoryService) CanRead(ctx context.Context, workspaceID, userID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := requireUser(userID); err != nil {
		return err
	}
	s.mu.RLock()
	store := s.store
	_, err := s.roleLocked(workspaceID, userID)
	s.mu.RUnlock()
	if err == nil {
		return nil
	}
	if membership, ok := store.(MembershipStore); ok {
		return membership.CanRead(ctx, workspaceID, userID)
	}
	return err
}

func (s *MemoryService) CanWrite(ctx context.Context, workspaceID, userID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := requireUser(userID); err != nil {
		return err
	}
	s.mu.RLock()
	store := s.store
	role, err := s.roleLocked(workspaceID, userID)
	s.mu.RUnlock()
	if err == nil {
		if role == RoleViewer {
			return ErrForbidden
		}
		return nil
	}
	if membership, ok := store.(MembershipStore); ok {
		return membership.CanWrite(ctx, workspaceID, userID)
	}
	return err
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else if b.Len() > 0 {
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}
