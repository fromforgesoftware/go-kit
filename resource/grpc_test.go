package resource_test

import (
	"testing"
	"time"

	resourcepb "github.com/fromforgesoftware/go-kit/proto/tb/v1"
	"github.com/fromforgesoftware/go-kit/ptr"
	"github.com/fromforgesoftware/go-kit/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToProtoFromProto(t *testing.T) {
	tests := []struct {
		name     string
		resource resource.Resource
	}{
		{
			name: "converts resource with all fields",
			resource: resource.New(
				resource.WithID("res-123"),
				resource.WithType("player"),
				resource.WithCreatedAt(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)),
				resource.WithUpdatedAt(time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC)),
			),
		},
		{
			name: "converts resource with soft delete",
			resource: resource.New(
				resource.WithID("res-456"),
				resource.WithType("guild"),
				resource.WithCreatedAt(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
				resource.WithUpdatedAt(time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)),
				resource.WithDeletedAt(ptr.Of(time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC))),
			),
		},
		{
			name: "converts resource with LID",
			resource: resource.New(
				resource.WithID("uuid-789"),
				resource.WithLID("123"),
				resource.WithType("item"),
				resource.WithCreatedAt(time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)),
				resource.WithUpdatedAt(time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC)),
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert to proto
			protoRes := resource.ToProto(tt.resource)
			require.NotNil(t, protoRes)

			// Verify proto fields
			assert.Equal(t, tt.resource.ID(), protoRes.GetId())
			assert.Equal(t, tt.resource.Type().String(), protoRes.GetType())
			assert.Equal(t, tt.resource.CreatedAt().Unix(), protoRes.GetCreatedAt().GetSeconds())
			assert.Equal(t, tt.resource.UpdatedAt().Unix(), protoRes.GetUpdatedAt().GetSeconds())

			// Convert back from proto
			convertedBack := resource.FromProto(protoRes)
			require.NotNil(t, convertedBack)

			// Verify roundtrip accuracy
			assert.Equal(t, tt.resource.ID(), convertedBack.ID())
			assert.Equal(t, tt.resource.Type(), convertedBack.Type())

			// Time comparison (within 1 second due to proto precision)
			assert.WithinDuration(t, tt.resource.CreatedAt(), convertedBack.CreatedAt(), time.Second)
			assert.WithinDuration(t, tt.resource.UpdatedAt(), convertedBack.UpdatedAt(), time.Second)

			// Check DeletedAt
			if tt.resource.DeletedAt() != nil {
				require.NotNil(t, convertedBack.DeletedAt())
				assert.WithinDuration(t, *tt.resource.DeletedAt(), *convertedBack.DeletedAt(), time.Second)
			} else {
				assert.Nil(t, convertedBack.DeletedAt())
			}
		})
	}
}

func TestToProtoNilHandling(t *testing.T) {
	t.Run("returns empty proto for nil resource", func(t *testing.T) {
		protoRes := resource.ToProto(nil)

		require.NotNil(t, protoRes)
		assert.Empty(t, protoRes.GetId())
		assert.Empty(t, protoRes.GetType())
	})
}

func TestFromProtoNilHandling(t *testing.T) {
	t.Run("returns nil for nil proto", func(t *testing.T) {
		res := resource.FromProto(nil)
		assert.Nil(t, res)
	})

	t.Run("returns nil for empty proto", func(t *testing.T) {
		res := resource.FromProto(&resourcepb.Resource{})

		require.NotNil(t, res)
		assert.Empty(t, res.ID())
		assert.Empty(t, res.Type())
	})
}

func TestIdentifierToProtoFromProto(t *testing.T) {
	tests := []struct {
		name       string
		identifier resource.Identifier
	}{
		{
			name:       "converts simple identifier",
			identifier: resource.NewIdentifier("player-123", "player"),
		},
		{
			name:       "converts identifier with empty id",
			identifier: resource.NewIdentifier("", "guild"),
		},
		{
			name:       "converts identifier with complex type",
			identifier: resource.NewIdentifier("item-456", "inventory.item"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert to proto
			protoId := resource.IdentifierToProto(tt.identifier)
			require.NotNil(t, protoId)
			assert.Equal(t, tt.identifier.ID(), protoId.GetId())
			assert.Equal(t, tt.identifier.Type().String(), protoId.GetType())

			// Convert back
			convertedBack := resource.IdentifierFromProto(protoId)
			require.NotNil(t, convertedBack)
			assert.Equal(t, tt.identifier.ID(), convertedBack.ID())
			assert.Equal(t, tt.identifier.Type(), convertedBack.Type())
		})
	}
}

func TestIdentifierToProtoNilHandling(t *testing.T) {
	t.Run("returns empty proto for nil identifier", func(t *testing.T) {
		protoId := resource.IdentifierToProto(nil)

		require.NotNil(t, protoId)
		assert.Empty(t, protoId.GetId())
		assert.Empty(t, protoId.GetType())
	})
}

func TestIdentifierFromProtoNilHandling(t *testing.T) {
	t.Run("returns nil for nil proto identifier", func(t *testing.T) {
		identifier := resource.IdentifierFromProto(nil)
		assert.Nil(t, identifier)
	})
}

func TestIdentifiersToProto(t *testing.T) {
	tests := []struct {
		name        string
		identifiers []resource.Identifier
		wantLen     int
	}{
		{
			name: "converts multiple identifiers",
			identifiers: []resource.Identifier{
				resource.NewIdentifier("id-1", "player"),
				resource.NewIdentifier("id-2", "guild"),
				resource.NewIdentifier("id-3", "item"),
			},
			wantLen: 3,
		},
		{
			name:        "converts empty slice",
			identifiers: []resource.Identifier{},
			wantLen:     0,
		},
		{
			name:        "converts nil slice",
			identifiers: nil,
			wantLen:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			protoIds := resource.IdentifiersToProto(tt.identifiers)

			require.NotNil(t, protoIds)
			assert.Len(t, protoIds, tt.wantLen)

			// Verify each conversion
			for i, identifier := range tt.identifiers {
				assert.Equal(t, identifier.ID(), protoIds[i].GetId())
				assert.Equal(t, identifier.Type().String(), protoIds[i].GetType())
			}
		})
	}
}

func TestDeletedAtTimestampConversion(t *testing.T) {
	t.Run("nil DeletedAt converts to nil timestamp", func(t *testing.T) {
		res := resource.New(
			resource.WithID("res-123"),
			resource.WithType("player"),
			resource.WithCreatedAt(time.Now()),
			resource.WithUpdatedAt(time.Now()),
		)

		protoRes := resource.ToProto(res)
		assert.Nil(t, protoRes.GetDeletedAt())

		convertedBack := resource.FromProto(protoRes)
		assert.Nil(t, convertedBack.DeletedAt())
	})

	t.Run("non-nil DeletedAt converts correctly", func(t *testing.T) {
		deletedAt := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
		res := resource.New(
			resource.WithID("res-456"),
			resource.WithType("guild"),
			resource.WithCreatedAt(time.Now()),
			resource.WithUpdatedAt(time.Now()),
			resource.WithDeletedAt(&deletedAt),
		)

		protoRes := resource.ToProto(res)
		require.NotNil(t, protoRes.GetDeletedAt())

		convertedBack := resource.FromProto(protoRes)
		require.NotNil(t, convertedBack.DeletedAt())
		assert.WithinDuration(t, deletedAt, *convertedBack.DeletedAt(), time.Second)
	})
}
