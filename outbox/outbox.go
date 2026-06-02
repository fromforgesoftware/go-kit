// Package outbox is the transactional-outbox primitive that lets a
// service emit at-least-once messages atomically with its local
// database writes. The producer Enqueues a Message inside the same
// gorm transaction as its INSERT/UPDATE; a separate process (Drainer
// for Cloud Run Jobs, Worker for long-running dev / VMs) later claims
// the rows, dispatches to a registered Handler, and marks them
// done/failed.
//
// Why exist: distributed writes that span "local DB + remote gRPC"
// can't be wrapped in a single transaction. Without the outbox a
// service either makes the gRPC call inside a tx (rolled back on
// failure but the remote side already saw it) or outside (and risks
// orphaning the local row if the gRPC fails). The outbox table is
// the durable handoff: write intent + row atomically, dispatch later
// with retries.
//
// Handlers MUST be idempotent. The drainer guarantees at-least-once
// delivery (replay on failure, on crash recovery, on concurrent
// drainer races). Use UNIQUE constraints on the remote side or check-
// before-write inside the handler.
//
// Cloud Run shape: ship a `cmd/outbox-drainer/` binary alongside
// `cmd/server/` and trigger it via Cloud Scheduler every 30s–1min.
// The Drainer makes one pass and exits — no goroutines, no CPU-on
// instances. For local dev under `make dev`, the Worker keeps a
// long-running loop in the same process.
package outbox

import (
	"context"
	"encoding/json"
	"time"
)

// Message is one outbox row. Kind selects a Handler; Payload is the
// handler-specific shape encoded as JSON so the table doesn't depend
// on producer/consumer types.
type Message struct {
	ID        string
	Kind      string
	Payload   json.RawMessage
	CreatedAt time.Time
	Attempts  int
	LastError string
}

// MessageDraft is what callers pass to Enqueue. The repository
// assigns the id, created_at, attempts=0.
type MessageDraft struct {
	Kind    string
	Payload json.RawMessage
}

// NewDraft is a small constructor that JSON-encodes the payload so
// call sites don't have to import encoding/json.
func NewDraft(kind string, payload any) (MessageDraft, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return MessageDraft{}, err
	}
	return MessageDraft{Kind: kind, Payload: b}, nil
}

// Repository is the persistence contract. Implementations:
//   - postgres.Repository (this package) — gorm/Postgres-backed.
//   - outboxtest.InMemory — for unit tests.
//
// Enqueue MUST run inside the producer's transaction so the outbox
// row commits atomically with the producer's INSERT. The kit
// persistence Transactioner ctx-propagates this automatically — pass
// the txCtx the Transactioner gives you.
type Repository interface {
	Enqueue(ctx context.Context, drafts ...MessageDraft) error
	// Claim grabs up to `batch` rows whose retry_at <= NOW(), tags
	// them as in-flight (FOR UPDATE SKIP LOCKED in postgres), and
	// returns them. Concurrent drainer replicas don't step on each
	// other.
	Claim(ctx context.Context, batch int) ([]Message, error)
	MarkDone(ctx context.Context, id string) error
	// MarkFailed bumps attempts, stores the error, and pushes retry_at
	// forward via exponential backoff. After maxAttempts the row is
	// moved to dead-letter status (caller queries those separately).
	MarkFailed(ctx context.Context, id string, handlerErr error, retryAt time.Time) error
}

// Handler processes one Message. Returning an error → the row is
// retried. Returning nil → marked done. Handlers MUST be idempotent.
type Handler interface {
	Handle(ctx context.Context, msg Message) error
}

// HandlerFunc adapts an ordinary function to Handler.
type HandlerFunc func(ctx context.Context, msg Message) error

func (f HandlerFunc) Handle(ctx context.Context, msg Message) error {
	return f(ctx, msg)
}
