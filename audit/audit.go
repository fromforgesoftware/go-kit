// Package audit defines a transport-neutral audit event and sink port so
// producers emit events without importing any sink backend.
package audit

import (
	"context"
	"time"
)

// Event is a structured record of a state-changing operation — the neutral
// shape shared by every producer and sink.
type Event struct {
	ID           string
	Timestamp    time.Time
	RealmID      string
	ActorID      string
	ActorType    string
	ResourceType string
	ResourceID   string
	Action       string
	Summary      string
	Changes      map[string]any
	Metadata     map[string]any
	IP           string
	RequestID    string
}

// Sink receives audit events. Implementations (Postgres, stdout, an outbox, a
// telemetry adapter) are swapped by configuration without touching emitters.
type Sink interface {
	Emit(ctx context.Context, e Event) error
}
