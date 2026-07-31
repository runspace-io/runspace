package workspace

import (
	"strconv"
	"strings"
)

func (s *MemoryService) hydrateRepositories(repositories []Repository) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, repository := range repositories {
		s.repositories[repository.ID] = repository
		if parts := strings.SplitN(repository.ID, "_", 2); len(parts) == 2 {
			if value, err := strconv.ParseUint(parts[1], 10, 64); err == nil && value > s.seq {
				s.seq = value
			}
		}
	}
}
