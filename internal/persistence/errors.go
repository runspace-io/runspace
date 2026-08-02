package persistence

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/runspace/runspace/internal/workspace"
)

// uniqueViolation is the PostgreSQL SQLSTATE for a duplicate key.
const uniqueViolation = "23505"

// alreadyExists translates a unique-violation into the domain conflict error so
// handlers answer 409 rather than leaking a driver error as a 500. In-memory
// duplicate checks only see rows this process has hydrated, so after a restart
// the database constraint is the first thing to notice a clash.
func alreadyExists(err error) error {
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) && pgError.Code == uniqueViolation {
		return workspace.ErrAlreadyExists
	}
	return err
}
