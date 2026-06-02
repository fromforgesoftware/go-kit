package postgres

import (
	"context"

	"github.com/fromforgesoftware/go-kit/application/repository"
	"gorm.io/gorm/clause"
)

type queryApplySetup struct {
	lock         *clause.Locking
	resourceType string
}

type queryApplyOption func(*queryApplySetup)

func WithResourceType(resourceType string) queryApplyOption {
	return func(s *queryApplySetup) {
		s.resourceType = resourceType
	}
}

// withLock applies database locking based on the lock information retrieved from the context.
// It checks the lock level and mode to determine the appropriate locking clause for the database query.
//
// If the conditions don't match the expected cases, the function panics with an error message
// indicating an unexpected locking mode or level.
//
// The function returns a copy of the repository with the applied database clause.
//
// For more information regarding PostgreSQL locks visit: https://www.postgresql.org/docs/14/explicit-locking.html
func withLock(ctx context.Context, tableName string) queryApplyOption {
	return func(s *queryApplySetup) {
		lock := repository.LockFromCtx(ctx)
		if lock == nil {
			return
		}

		if lock.Level() != repository.LockLevelRow {
			// We only support row level locking for now
			return
		}

		strength := ""
		if lock.Contains(repository.LockModeExclusive) {
			strength = "UPDATE"
		} else if lock.Contains(repository.LockModeShare) {
			strength = "SHARE"
		}

		if strength != "" {
			s.lock = &clause.Locking{
				Strength: strength,
				Table:    clause.Table{Name: tableName},
			}
		}
	}
}
