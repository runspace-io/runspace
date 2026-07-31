package runs

import "context"

func (s *Service) ListOutputs(ctx context.Context, id string) ([]Output, error) {
	s.mu.RLock()
	store := s.store
	s.mu.RUnlock()
	if store != nil {
		return store.ListOutputs(ctx, id)
	}
	return []Output{}, nil
}

func (s *Service) ListRuns(ctx context.Context, threadID string) ([]Run, error) {
	s.mu.RLock()
	store := s.store
	local := make([]Run, 0)
	for _, run := range s.runs {
		if run.ThreadID == threadID {
			local = append(local, run)
		}
	}
	s.mu.RUnlock()
	if store != nil {
		return store.ListRuns(ctx, threadID)
	}
	return local, nil
}
