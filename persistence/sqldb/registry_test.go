//go:build integration
// +build integration

package sqldb_test

import (
	"context"
	"testing"

	"github.com/fromforgesoftware/go-kit/persistence/sqldb"
	"github.com/fromforgesoftware/go-kit/persistence/sqldb/sqldbtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistryIntegration(t *testing.T) {
	// Use integration DB
	testDB := sqldbtest.GetDB(t, sqldbtest.TestSchema)
	db := testDB.DB()

	registry := sqldb.NewRegistry(db)
	require.NotNil(t, registry)
	defer registry.Close()

	ctx := context.Background()
	id := sqldb.StatementID("select_one")
	query := "SELECT 1" // Simple valid query

	t.Run("Register success", func(t *testing.T) {
		err := registry.Register(ctx, id, query)
		require.NoError(t, err)
		assert.Equal(t, 1, registry.Count())
	})

	t.Run("Register duplicate", func(t *testing.T) {
		// Already registered in previous test
		err := registry.Register(ctx, id, query)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already registered")
	})

	t.Run("Get success", func(t *testing.T) {
		stmt, err := registry.Get(id)
		require.NoError(t, err)
		assert.NotNil(t, stmt)

		// Verify statement works
		var result int
		err = stmt.QueryRow().Scan(&result)
		require.NoError(t, err)
		assert.Equal(t, 1, result)
	})

	t.Run("Get not found", func(t *testing.T) {
		stmt, err := registry.Get("unknown")
		require.Error(t, err)
		assert.Nil(t, stmt)
		assert.Contains(t, err.Error(), "not registered")
	})

	t.Run("MustRegister success", func(t *testing.T) {
		id2 := sqldb.StatementID("select_two")
		query2 := "SELECT 2"

		assert.NotPanics(t, func() {
			registry.MustRegister(ctx, id2, query2)
		})
		assert.Equal(t, 2, registry.Count())
	})

	t.Run("MustGet success", func(t *testing.T) {
		stmt := registry.MustGet(id)
		assert.NotNil(t, stmt)
	})

	t.Run("MustGet panic", func(t *testing.T) {
		assert.Panics(t, func() {
			registry.MustGet("unknown")
		})
	})

	// Close happens in defer, validated by lack of error in test cleanup typically,
	// but we can call it explicitly for testing.
	t.Run("Close success", func(t *testing.T) {
		err := registry.Close()
		require.NoError(t, err)
		// Try to use stmt after close -> should fail
		// or Re-closing?
	})
}
