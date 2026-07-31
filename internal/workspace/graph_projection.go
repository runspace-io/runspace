package workspace

import (
	"context"
	"time"
)

type GraphProjector interface {
	ProjectResource(context.Context, string, string, string, string, string, time.Time) error
}

func (s *MemoryService) SetGraphProjector(graph GraphProjector) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.graph = graph
}

func (s *MemoryService) projectResource(ctx context.Context, resource Resource) {
	if s.graph == nil {
		return
	}
	_ = s.graph.ProjectResource(
		ctx, resource.ID, resource.WorkspaceID, resource.CreatedBy,
		resource.Provider, resource.FullName, resource.CreatedAt,
	)
}
