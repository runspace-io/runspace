package collaboration

import (
	"context"
	"strings"
)

func (s *MemoryService) CreateMessage(ctx context.Context, userID, workspaceID, threadID, actorType, body string) (Message, error) {
	return s.createMessageAs(ctx, userID, userID, workspaceID, threadID, actorType, body)
}

func (s *MemoryService) CreateAgentMessage(
	ctx context.Context, callerID, agentID, workspaceID, threadID, body string,
) (Message, error) {
	return s.createMessageAs(ctx, callerID, agentID, workspaceID, threadID, "agent", body)
}

func (s *MemoryService) CreateAgentActivity(
	ctx context.Context, callerID, agentID, workspaceID, threadID, body string,
) (Message, error) {
	return s.createMessageAs(ctx, callerID, agentID, workspaceID, threadID, "activity", body)
}

func (s *MemoryService) createMessageAs(
	ctx context.Context, callerID, actorID, workspaceID, threadID, actorType, body string,
) (Message, error) {
	if err := ctx.Err(); err != nil {
		return Message{}, err
	}
	if invalidMessageInput(callerID, workspaceID, threadID, body) || strings.TrimSpace(actorID) == "" {
		return Message{}, ErrInvalid
	}
	if s.authorizer == nil {
		return Message{}, ErrUnauthorized
	}
	if err := s.authorizer.CanWrite(ctx, workspaceID, callerID); err != nil {
		return Message{}, err
	}
	thread, err := s.resolveThread(ctx, callerID, workspaceID, threadID)
	if err != nil {
		return Message{}, err
	}
	if thread.HiddenFrom(callerID) {
		return Message{}, ErrNotFound
	}
	s.mu.Lock()
	store := s.store
	now := s.clock().UTC()
	message := Message{ID: s.id("message"), ThreadID: threadID, ActorID: actorID, ActorType: strings.TrimSpace(actorType), Body: strings.TrimSpace(body), CreatedAt: now}
	s.messages[threadID] = append(s.messages[threadID], message)
	s.mu.Unlock()
	if store != nil {
		if err := store.CreateMessage(ctx, message); err != nil {
			return Message{}, err
		}
	}
	if err := s.publish(ctx, "message.created", workspaceID, actorID, message.ActorType, message); err != nil {
		return Message{}, err
	}
	return message, nil
}

func (s *MemoryService) resolveThread(ctx context.Context, userID, workspaceID, threadID string) (Thread, error) {
	s.mu.RLock()
	thread, ok := s.threads[threadID]
	store := s.store
	s.mu.RUnlock()
	if ok && thread.WorkspaceID == workspaceID {
		return thread, nil
	}
	if store == nil {
		return Thread{}, ErrNotFound
	}
	threads, err := store.ListThreads(ctx, userID, workspaceID)
	if err != nil {
		return Thread{}, err
	}
	for _, candidate := range threads {
		if candidate.ID == threadID {
			return candidate, nil
		}
	}
	return Thread{}, ErrNotFound
}

func invalidMessageInput(userID, workspaceID, threadID, body string) bool {
	return strings.TrimSpace(userID) == "" || strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(threadID) == "" || strings.TrimSpace(body) == ""
}

func (s *MemoryService) ListMessages(ctx context.Context, userID, workspaceID, threadID string) ([]Message, error) {
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
	defer s.mu.RUnlock()
	if store != nil {
		if persisted, err := store.ListMessages(ctx, userID, workspaceID, threadID); err == nil && persisted != nil {
			return ensureMessages(persisted), nil
		}
	}
	thread, ok := s.threads[threadID]
	if !ok || thread.WorkspaceID != workspaceID || thread.HiddenFrom(userID) {
		return nil, ErrNotFound
	}
	return ensureMessages(s.messages[threadID]), nil
}

func ensureMessages(messages []Message) []Message {
	if len(messages) == 0 {
		return []Message{}
	}
	return append([]Message(nil), messages...)
}
