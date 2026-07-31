package collaboration

import (
	"context"
	"sort"
	"strings"
)

// ChannelPatch contains the mutable channel settings. Nil fields preserve the
// current value, while a non-nil config replaces only explicitly supplied
// keys and remains layered over the parent configuration.
type ChannelPatch struct {
	Name          *string        `json:"name"`
	ResourceID    *string        `json:"resource_id"`
	ResourceIDs   *[]string      `json:"resource_ids"`
	RepositoryID  *string        `json:"repository_id"`
	RepositoryIDs *[]string      `json:"repository_ids"`
	Config        map[string]any `json:"config"`
}

func (s *MemoryService) CreateChannel(ctx context.Context, userID, workspaceID, name, parentID, repositoryID string, config map[string]any) (Channel, error) {
	ids := []string{}
	if strings.TrimSpace(repositoryID) != "" {
		ids = append(ids, strings.TrimSpace(repositoryID))
	}
	return s.CreateChannelWithRepositories(ctx, userID, workspaceID, name, parentID, ids, config)
}

func (s *MemoryService) CreateChannelWithRepositories(ctx context.Context, userID, workspaceID, name, parentID string, repositoryIDs []string, config map[string]any) (Channel, error) {
	if err := s.validateChannelCreate(ctx, userID, workspaceID, name); err != nil {
		return Channel{}, err
	}
	parentID = strings.TrimSpace(parentID)
	parent, hasParent := s.resolveParent(ctx, userID, workspaceID, parentID)
	s.mu.Lock()
	if invalidParent(parentID, workspaceID, parent, hasParent) {
		s.mu.Unlock()
		return Channel{}, ErrNotFound
	}
	repositoryIDs = inheritedRepositoryIDs(repositoryIDs, parent, hasParent)
	effective := cloneConfig(config)
	if hasParent {
		effective = mergeConfig(parent.Config, effective)
	}
	now := s.clock().UTC()
	legacyID := ""
	if len(repositoryIDs) > 0 {
		legacyID = repositoryIDs[0]
	}
	channel := Channel{ID: s.id("channel"), WorkspaceID: workspaceID, Name: strings.TrimSpace(name), ParentID: parentID, RepositoryID: legacyID, RepositoryIDs: repositoryIDs, Config: effective, CreatedBy: userID, CreatedAt: now}
	s.channels[channel.ID] = channel
	store := s.store
	s.mu.Unlock()
	if channelStore, ok := store.(ChannelStore); ok {
		if err := channelStore.CreateCollaborationChannel(ctx, channel); err != nil {
			return Channel{}, err
		}
	}
	if err := s.publish(ctx, "channel.created", workspaceID, userID, "user", channel); err != nil {
		return Channel{}, err
	}
	return channel, nil
}

func (s *MemoryService) UpdateChannel(ctx context.Context, userID, workspaceID, channelID string, patch ChannelPatch) (Channel, error) {
	if err := s.validateChannelAccess(ctx, userID, workspaceID); err != nil {
		return Channel{}, err
	}
	channel, err := s.loadWritableChannel(ctx, userID, workspaceID, channelID)
	if err != nil {
		return Channel{}, err
	}
	if err := s.applyChannelPatch(ctx, userID, workspaceID, &channel, patch); err != nil {
		return Channel{}, err
	}
	s.mu.Lock()
	s.channels[channel.ID] = channel
	store := s.store
	s.mu.Unlock()
	if channelStore, ok := store.(ChannelStore); ok {
		if err := channelStore.UpdateCollaborationChannel(ctx, channel); err != nil {
			return Channel{}, err
		}
	}
	if err := s.publish(ctx, "channel.updated", workspaceID, userID, "user", channel); err != nil {
		return Channel{}, err
	}
	return channel, nil
}

func (s *MemoryService) loadWritableChannel(ctx context.Context, userID, workspaceID, channelID string) (Channel, error) {
	channel, err := s.GetChannel(ctx, userID, channelID)
	if err != nil {
		return Channel{}, err
	}
	if channel.WorkspaceID != workspaceID {
		return Channel{}, ErrNotFound
	}
	return channel, nil
}

func (s *MemoryService) applyChannelPatch(ctx context.Context, userID, workspaceID string, channel *Channel, patch ChannelPatch) error {
	if patch.Name != nil {
		channel.Name = strings.TrimSpace(*patch.Name)
		if channel.Name == "" {
			return ErrInvalid
		}
	}
	applyRepositoryPatch(channel, patch)
	parent, hasParent := s.resolveParent(ctx, userID, workspaceID, channel.ParentID)
	if len(channel.RepositoryIDs) == 0 && channel.RepositoryID == "" && hasParent {
		channel.RepositoryIDs = inheritedRepositoryIDs(nil, parent, true)
		if len(channel.RepositoryIDs) > 0 {
			channel.RepositoryID = channel.RepositoryIDs[0]
		}
	}
	if patch.Config != nil {
		channel.Config = updatedChannelConfig(parent, hasParent, patch.Config)
	}
	return nil
}

func updatedChannelConfig(parent Channel, hasParent bool, patch map[string]any) map[string]any {
	if hasParent {
		return mergeConfig(parent.Config, patch)
	}
	return cloneConfig(patch)
}

func (s *MemoryService) validateChannelAccess(ctx context.Context, userID, workspaceID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(workspaceID) == "" {
		return ErrInvalid
	}
	if s.authorizer == nil {
		return ErrUnauthorized
	}
	return s.authorizer.CanWrite(ctx, workspaceID, userID)
}

func (s *MemoryService) resolveParent(ctx context.Context, userID, workspaceID, parentID string) (Channel, bool) {
	s.mu.RLock()
	parent, found := s.channels[parentID]
	store := s.store
	s.mu.RUnlock()
	if parentID != "" && !found {
		parent, found = findPersistedParent(ctx, store, userID, workspaceID, parentID)
	}
	return parent, found
}

func findPersistedParent(ctx context.Context, store Store, userID, workspaceID, parentID string) (Channel, bool) {
	channelStore, ok := store.(ChannelStore)
	if !ok {
		return Channel{}, false
	}
	channels, err := channelStore.ListCollaborationChannels(ctx, userID, workspaceID)
	if err != nil {
		return Channel{}, false
	}
	for _, candidate := range channels {
		if candidate.ID == parentID {
			return candidate, true
		}
	}
	return Channel{}, false
}

func (s *MemoryService) validateChannelCreate(ctx context.Context, userID, workspaceID, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if invalidChannelInput(userID, workspaceID, name) {
		return ErrInvalid
	}
	if s.authorizer == nil {
		return ErrUnauthorized
	}
	return s.authorizer.CanWrite(ctx, workspaceID, userID)
}

func invalidChannelInput(userID, workspaceID, name string) bool {
	return strings.TrimSpace(userID) == "" || strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(name) == ""
}

func invalidParent(parentID, workspaceID string, parent Channel, exists bool) bool {
	return parentID != "" && (!exists || parent.WorkspaceID != workspaceID)
}

func (s *MemoryService) ListChannels(ctx context.Context, userID, workspaceID string) ([]Channel, error) {
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
	if channelStore, ok := store.(ChannelStore); ok {
		if persisted, err := channelStore.ListCollaborationChannels(ctx, userID, workspaceID); err == nil && persisted != nil {
			return persisted, nil
		}
	}
	result := make([]Channel, 0)
	for _, channel := range s.channels {
		if channel.WorkspaceID == workspaceID {
			channel.Config = cloneConfig(channel.Config)
			result = append(result, channel)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.Before(result[j].CreatedAt) })
	return result, nil
}

func (s *MemoryService) GetChannel(ctx context.Context, userID, channelID string) (Channel, error) {
	if err := ctx.Err(); err != nil {
		return Channel{}, err
	}
	s.mu.RLock()
	channel, ok := s.channels[channelID]
	store := s.store
	s.mu.RUnlock()
	if !ok {
		channel, ok = loadChannel(ctx, store, userID, channelID)
	}
	if !ok {
		return Channel{}, ErrNotFound
	}
	if s.authorizer == nil {
		return Channel{}, ErrUnauthorized
	}
	if err := s.authorizer.CanRead(ctx, channel.WorkspaceID, userID); err != nil {
		return Channel{}, err
	}
	channel.Config = cloneConfig(channel.Config)
	return channel, nil
}

func loadChannel(ctx context.Context, store Store, userID, channelID string) (Channel, bool) {
	channelStore, ok := store.(ChannelStore)
	if !ok {
		return Channel{}, false
	}
	channels, err := channelStore.ListCollaborationChannels(ctx, userID, "")
	if err != nil {
		return Channel{}, false
	}
	for _, candidate := range channels {
		if candidate.ID == channelID {
			return candidate, true
		}
	}
	return Channel{}, false
}

func cloneConfig(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func mergeConfig(parent, child map[string]any) map[string]any {
	output := cloneConfig(parent)
	if output == nil {
		output = make(map[string]any)
	}
	for key, value := range child {
		output[key] = value
	}
	return output
}
