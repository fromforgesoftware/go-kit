package resource_test

import (
	"testing"
	"time"

	"github.com/fromforgesoftware/go-kit/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Run("creates resource with all fields", func(t *testing.T) {
		res := resource.New(
			resource.WithID("res-123"),
			resource.WithLID("1"),
			resource.WithType("player"),
			resource.WithCreatedAt(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
			resource.WithUpdatedAt(time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)),
		)

		require.NotNil(t, res)
		assert.Equal(t, "res-123", res.ID())
		assert.Equal(t, "1", res.LID())
		assert.Equal(t, resource.Type("player"), res.Type())
		assert.Equal(t, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), res.CreatedAt())
		assert.Equal(t, time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), res.UpdatedAt())
		assert.Nil(t, res.DeletedAt())
	})

	t.Run("creates resource with soft delete", func(t *testing.T) {
		deletedAt := time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)
		res := resource.New(
			resource.WithID("res-456"),
			resource.WithType("guild"),
			resource.WithDeletedAt(&deletedAt),
		)

		require.NotNil(t, res)
		assert.Equal(t, "res-456", res.ID())
		require.NotNil(t, res.DeletedAt())
		assert.Equal(t, deletedAt, *res.DeletedAt())
	})

	t.Run("creates empty resource", func(t *testing.T) {
		res := resource.New()

		require.NotNil(t, res)
		assert.Empty(t, res.ID())
		assert.Empty(t, res.LID())
		assert.Empty(t, res.Type())
		assert.Nil(t, res.DeletedAt())
	})
}

func TestUpdate(t *testing.T) {
	t.Run("updates timestamp while preserving other fields", func(t *testing.T) {
		original := resource.New(
			resource.WithID("res-123"),
			resource.WithLID("1"),
			resource.WithType("player"),
			resource.WithCreatedAt(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
			resource.WithUpdatedAt(time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)),
		)

		updated := resource.Update(original,
			resource.WithUpdatedAt(time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)),
		)

		require.NotNil(t, updated)
		assert.Equal(t, "res-123", updated.ID())
		assert.Equal(t, "1", updated.LID())
		assert.Equal(t, resource.Type("player"), updated.Type())
		assert.Equal(t, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), updated.CreatedAt())
		assert.Equal(t, time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC), updated.UpdatedAt())
	})

	t.Run("soft deletes resource", func(t *testing.T) {
		original := resource.New(
			resource.WithID("res-456"),
			resource.WithType("guild"),
		)

		deletedAt := time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC)
		updated := resource.Update(original,
			resource.WithDeletedAt(&deletedAt),
		)

		require.NotNil(t, updated)
		require.NotNil(t, updated.DeletedAt())
		assert.Equal(t, deletedAt, *updated.DeletedAt())
	})

	t.Run("updates multiple fields", func(t *testing.T) {
		original := resource.New(
			resource.WithID("old-id"),
			resource.WithType("item"),
		)

		updated := resource.Update(original,
			resource.WithID("new-id"),
			resource.WithLID("999"),
			resource.WithUpdatedAt(time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)),
		)

		require.NotNil(t, updated)
		assert.Equal(t, "new-id", updated.ID())
		assert.Equal(t, "999", updated.LID())
		assert.Equal(t, resource.Type("item"), updated.Type())
		assert.Equal(t, time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC), updated.UpdatedAt())
	})
}

func TestNewIdentifier(t *testing.T) {
	t.Run("creates identifier with string type", func(t *testing.T) {
		identifier := resource.NewIdentifier("player-123", "player")

		require.NotNil(t, identifier)
		assert.Equal(t, "player-123", identifier.ID())
		assert.Equal(t, resource.Type("player"), identifier.Type())
	})

	t.Run("creates identifier with empty id", func(t *testing.T) {
		identifier := resource.NewIdentifier("", "guild")

		require.NotNil(t, identifier)
		assert.Empty(t, identifier.ID())
		assert.Equal(t, resource.Type("guild"), identifier.Type())
	})
}

func TestTypeString(t *testing.T) {
	typ := resource.Type("player")
	assert.Equal(t, "player", typ.String())
}
