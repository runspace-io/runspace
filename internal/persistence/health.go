package persistence

import (
	"context"
	"fmt"
)

func (s *Store) Ping(ctx context.Context) error {
	if s.db == nil {
		return fmt.Errorf("database is nil")
	}
	return s.db.PingContext(ctx)
}
