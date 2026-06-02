package gormdb

import (
	"context"

	"gorm.io/gorm"
)

// AcquireAdvisoryLock acquires a PostgreSQL advisory lock using the provided transaction.
func AcquireAdvisoryLock(ctx context.Context, tx *gorm.DB, lockID int) error {
	return tx.WithContext(ctx).Exec("SELECT pg_advisory_lock(?);", lockID).Error
}

// ReleaseAdvisoryLock releases a PostgreSQL advisory lock using the provided transaction.
func ReleaseAdvisoryLock(ctx context.Context, tx *gorm.DB, lockID int) error {
	return tx.WithContext(ctx).Exec("SELECT pg_advisory_unlock(?);", lockID).Error
}
