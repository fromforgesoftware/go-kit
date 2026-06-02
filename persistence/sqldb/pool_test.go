//go:build integration
// +build integration

package sqldb_test

import (
	"testing"
	"time"

	"github.com/fromforgesoftware/go-kit/persistence/sqldb"
	"github.com/fromforgesoftware/go-kit/persistence/sqldb/sqldbtest"
	"github.com/stretchr/testify/assert"
)

func TestPoolOptionsIntegration(t *testing.T) {
	testDB := sqldbtest.GetDB(t, sqldbtest.TestSchema)
	db := testDB.DB()

	// We can verify that applying options doesn't panic on a real DB connection.
	// Verifying values:

	t.Run("ConfigurePool default", func(t *testing.T) {
		sqldb.ConfigurePool(db, sqldb.DefaultPoolOptions()...)

		stats := db.Stats()
		assert.Equal(t, 25, stats.MaxOpenConnections)
	})

	t.Run("ConfigurePool high concurrency", func(t *testing.T) {
		sqldb.ConfigurePool(db, sqldb.HighConcurrencyPoolOptions()...)

		stats := db.Stats()
		assert.Equal(t, 100, stats.MaxOpenConnections)
	})

	t.Run("Custom Options", func(t *testing.T) {
		sqldb.ConfigurePool(db,
			sqldb.WithPoolMaxOpenConns(50),
			sqldb.WithPoolMaxIdleConns(10),
			sqldb.WithPoolConnMaxLifetime(1*time.Hour),
			sqldb.WithPoolConnMaxIdleTime(30*time.Minute),
		)

		stats := db.Stats()
		assert.Equal(t, 50, stats.MaxOpenConnections)
	})
}
