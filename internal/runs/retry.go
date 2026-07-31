package runs

import (
	"context"
	"errors"

	"github.com/runspace/runspace/internal/contracts"
)

func (s *Service) Retry(ctx context.Context, id, newID string) (Run, error) {
	old, err := s.Get(ctx, id)
	if err != nil {
		return Run{}, err
	}
	if newID == "" {
		return Run{}, errors.New("new run ID required")
	}
	return s.Create(ctx, retrySpawnRequest(old, newID))
}

func retrySpawnRequest(old Run, newID string) contracts.SpawnRequest {
	return contracts.SpawnRequest{
		RunID:            newID,
		WorkspaceID:      old.WorkspaceID,
		ThreadID:         old.ThreadID,
		ChannelID:        old.ChannelID,
		Repository:       old.RepositoryID,
		Prompt:           old.Prompt,
		AgentCommand:     old.AgentCommand,
		WorkingDirectory: old.WorkingDirectory,
	}
}
