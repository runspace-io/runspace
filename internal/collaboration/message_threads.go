package collaboration

import (
	"context"
	"sort"
	"strings"
)

// CreateMessageThread starts (or, idempotently, returns) the subthread
// anchored to parentMessageID inside parentThreadID. At most one public
// subthread exists per message; each user may additionally have their own
// private subthread on the same message.
func (s *MemoryService) CreateMessageThread(
	ctx context.Context, userID, workspaceID, parentThreadID, parentMessageID, visibility string,
) (Thread, error) {
	if err := ctx.Err(); err != nil {
		return Thread{}, err
	}
	visibility = normalizeThreadVisibility(visibility)
	if invalidMessageThreadInput(userID, workspaceID, parentThreadID, parentMessageID) || !validThreadVisibility(visibility) {
		return Thread{}, ErrInvalid
	}
	if s.authorizer == nil {
		return Thread{}, ErrUnauthorized
	}
	if err := s.authorizer.CanWrite(ctx, workspaceID, userID); err != nil {
		return Thread{}, err
	}
	parentThread, err := s.resolveThread(ctx, userID, workspaceID, parentThreadID)
	if err != nil {
		return Thread{}, err
	}
	existing, found, err := s.findMessageThread(ctx, workspaceID, parentThreadID, parentMessageID, visibility, userID)
	if err != nil {
		return Thread{}, err
	}
	if found {
		return existing, nil
	}
	s.mu.Lock()
	now := s.clock().UTC()
	thread := Thread{
		ID:              s.id("thread"),
		WorkspaceID:     workspaceID,
		ChannelID:       parentThread.ChannelID,
		ParentThreadID:  parentThreadID,
		ParentMessageID: parentMessageID,
		Visibility:      visibility,
		CreatedBy:       userID,
		CreatedAt:       now,
	}
	s.threads[thread.ID] = thread
	store := s.store
	s.mu.Unlock()
	if err := s.persistThread(ctx, store, thread); err != nil {
		return Thread{}, err
	}
	// Deliberately not graph-projected: a private subthread must never leak
	// its existence through the workspace knowledge graph.
	return thread, nil
}

// ListMessageThreads returns every subthread anchored to a message inside
// parentThreadID that userID may see: every public one, plus userID's own
// private ones.
func (s *MemoryService) ListMessageThreads(
	ctx context.Context, userID, workspaceID, parentThreadID string,
) ([]Thread, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.authorizer == nil {
		return nil, ErrUnauthorized
	}
	if err := s.authorizer.CanRead(ctx, workspaceID, userID); err != nil {
		return nil, err
	}
	all, err := s.listMessageThreadsRaw(ctx, workspaceID, parentThreadID)
	if err != nil {
		return nil, err
	}
	visible := make([]Thread, 0, len(all))
	for _, thread := range all {
		if !thread.HiddenFrom(userID) {
			visible = append(visible, thread)
		}
	}
	sort.Slice(visible, func(i, j int) bool { return visible[i].CreatedAt.Before(visible[j].CreatedAt) })
	return visible, nil
}

// ListPrivateThreads lists every private subthread userID created, across
// the workspace, for that user's private-threads tab.
func (s *MemoryService) ListPrivateThreads(ctx context.Context, userID, workspaceID string) ([]Thread, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.authorizer == nil {
		return nil, ErrUnauthorized
	}
	if err := s.authorizer.CanRead(ctx, workspaceID, userID); err != nil {
		return nil, err
	}
	s.mu.RLock()
	store := s.store
	s.mu.RUnlock()
	if store != nil {
		threads, err := store.ListThreadsByCreator(ctx, workspaceID, userID, ThreadVisibilityPrivate)
		if err != nil {
			return nil, err
		}
		return threads, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Thread, 0)
	for _, thread := range s.threads {
		if thread.WorkspaceID == workspaceID && thread.CreatedBy == userID &&
			thread.Visibility == ThreadVisibilityPrivate && thread.ParentMessageID != "" {
			result = append(result, thread)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.Before(result[j].CreatedAt) })
	return result, nil
}

// listMessageThreadsRaw returns every subthread under parentThreadID
// regardless of visibility or viewer; callers apply their own filtering.
func (s *MemoryService) listMessageThreadsRaw(ctx context.Context, workspaceID, parentThreadID string) ([]Thread, error) {
	s.mu.RLock()
	store := s.store
	s.mu.RUnlock()
	if store != nil {
		return store.ListThreadsByParentThreadID(ctx, workspaceID, parentThreadID)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Thread, 0)
	for _, thread := range s.threads {
		if thread.WorkspaceID == workspaceID && thread.ParentThreadID == parentThreadID {
			result = append(result, thread)
		}
	}
	return result, nil
}

// findMessageThread looks for a thread matching (parentMessageID, visibility)
// that userID may reuse: the single public one if it exists, or userID's own
// private one.
func (s *MemoryService) findMessageThread(
	ctx context.Context, workspaceID, parentThreadID, parentMessageID, visibility, userID string,
) (Thread, bool, error) {
	candidates, err := s.listMessageThreadsRaw(ctx, workspaceID, parentThreadID)
	if err != nil {
		return Thread{}, false, err
	}
	for _, candidate := range candidates {
		if candidate.ParentMessageID != parentMessageID || candidate.Visibility != visibility {
			continue
		}
		if visibility == ThreadVisibilityPrivate && candidate.CreatedBy != userID {
			continue
		}
		return candidate, true, nil
	}
	return Thread{}, false, nil
}

func normalizeThreadVisibility(visibility string) string {
	visibility = strings.TrimSpace(strings.ToLower(visibility))
	if visibility == "" {
		return ThreadVisibilityPublic
	}
	return visibility
}

func validThreadVisibility(visibility string) bool {
	return visibility == ThreadVisibilityPublic || visibility == ThreadVisibilityPrivate
}

func invalidMessageThreadInput(userID, workspaceID, parentThreadID, parentMessageID string) bool {
	return strings.TrimSpace(userID) == "" || strings.TrimSpace(workspaceID) == "" ||
		strings.TrimSpace(parentThreadID) == "" || strings.TrimSpace(parentMessageID) == ""
}
