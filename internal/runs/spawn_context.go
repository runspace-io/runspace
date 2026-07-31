package runs

import (
	"context"
	"errors"
	"strings"

	"github.com/runspace/runspace/internal/contracts"
	"github.com/runspace/runspace/internal/workspace"
)

func (h *Handler) resolveSpawnContext(
	ctx context.Context,
	userID string,
	spawn *contracts.SpawnRequest,
) error {
	if h.chat == nil {
		return nil
	}
	threads, err := h.chat.ListThreads(ctx, userID, spawn.WorkspaceID)
	if err != nil {
		return err
	}
	for _, thread := range threads {
		if thread.ID == spawn.ThreadID {
			spawn.ChannelID = thread.ChannelID
			break
		}
	}
	if spawn.ChannelID == "" {
		return nil
	}
	channel, err := h.chat.GetChannel(ctx, userID, spawn.ChannelID)
	if err != nil {
		return err
	}
	spawn.AgentCommand = channelAgentCommand(channel.Config)
	return nil
}

func (h *Handler) assignWorkingDirectory(
	ctx context.Context,
	userID string,
	spawn *contracts.SpawnRequest,
) error {
	if strings.TrimSpace(spawn.Repository) == "" {
		return nil
	}
	if h.resolver == nil {
		return errors.New("resource resolver unavailable")
	}
	if h.authorizer == nil {
		return workspace.ErrUnauthorized
	}
	repositories, err := h.authorizer.ListRepositories(ctx, userID, spawn.WorkspaceID)
	if err != nil {
		return err
	}
	for _, repository := range repositories {
		if repository.ID == spawn.Repository {
			root, rootErr := h.resolver.Root(ctx, spawn.WorkspaceID, spawn.Repository)
			spawn.WorkingDirectory = root
			return rootErr
		}
	}
	return errors.New("resource is not connected to workspace")
}

func channelAgentCommand(config map[string]any) string {
	if agent, ok := config["agent"].(map[string]any); ok {
		if command, ok := agent["command"].(string); ok {
			return strings.TrimSpace(command)
		}
	}
	if command, ok := config["agent.command"].(string); ok {
		return strings.TrimSpace(command)
	}
	return ""
}
