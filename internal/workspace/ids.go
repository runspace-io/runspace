package workspace

import "fmt"

// nextID mints an identifier that does not depend on process-local state.
//
// The counter alone restarts at zero with the process, so with a database
// attached the first insert after a gateway restart reused an ID that was
// already persisted and failed on the primary key. The timestamp makes IDs
// unique across restarts; the counter keeps them unique within a clock tick.
//
// Callers must hold s.mu: the counter is shared across every prefix.
func (s *MemoryService) nextID(prefix string) string {
	s.seq++
	return fmt.Sprintf("%s_%d_%d", prefix, s.clock().UTC().UnixNano(), s.seq)
}
