// Package resource provides base abstractions for domain entities.
//
// This package defines the core Resource interface that all domain entities should implement.
// It separates concerns into smaller interfaces (Identifier, Timestamps) for flexibility
// and provides utilities for working with resources across different transport layers.
//
// # Core Interfaces
//
// Resource combines identification and timestamp tracking:
//   - Identifier: ID(), LID() (local ID), Type()
//   - Timestamps: CreatedAt(), UpdatedAt(), DeletedAt() (soft delete support)
//
// # Usage
//
// Creating a new resource:
//
//	player := resource.New(
//	    resource.WithID(uuid.New().String()),
//	    resource.WithType("player"),
//	    resource.WithCreatedAt(time.Now()),
//	    resource.WithUpdatedAt(time.Now()),
//	)
//
// Creating an identifier reference (lightweight):
//
//	playerRef := resource.NewIdentifier("player-123", resource.Type("player"))
//
// Updating a resource:
//
//	updated := resource.Update(player,
//	    resource.WithUpdatedAt(time.Now()),
//	)
//
// # Protocol Buffer Support
//
// Convert to/from protobuf for gRPC services:
//
//	// To proto
//	pb := resource.ToProto(player)
//
//	// From proto
//	player := resource.FromProto(pb)
//
//	// Identifiers
//	identifierPb := resource.IdentifierToProto(playerRef)
//
// # Testing
//
// Use resourcetest package for creating test fixtures:
//
//	import "github.com/fromforgesoftware/go-kit/resource/resourcetest"
//
//	testPlayer := resourcetest.New(
//	    resourcetest.WithType("player"),
//	    resourcetest.WithID("player-123"),
//	)
//
// # Soft Deletes
//
// Resources support soft deletion via the DeletedAt timestamp:
//
//	now := time.Now()
//	deleted := resource.Update(player,
//	    resource.WithDeletedAt(&now),
//	)
//
//	if deleted.DeletedAt() != nil {
//	    // Resource is soft-deleted
//	}
//
// # Type Safety
//
// Use strongly-typed resource types to avoid string errors:
//
//	const (
//	    TypePlayer = resource.Type("player")
//	    TypeGuild  = resource.Type("guild")
//	    TypeItem   = resource.Type("item")
//	)
package resource
