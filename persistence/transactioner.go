// Package persistence holds the database-agnostic primitives — Transactioner
// for atomic write paths, repository interfaces, and shared error mapping
// — that the concrete adapters (gormdb, sqldb, redisdb) implement.
package persistence

import "context"

// TxFunc is a function that runs within a transaction
type TxFunc func(ctx context.Context) error

// Transactioner is an interface for managing transactions
type Transactioner interface {
	// Exec executes a function within a transaction
	// If the function returns an error, the transaction is rolled back
	// Otherwise, the transaction is committed
	// Supports nested transactions by reusing existing transaction in context
	Exec(ctx context.Context, fn TxFunc) error
}
