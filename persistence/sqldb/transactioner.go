package sqldb

import (
	"context"
	"database/sql"

	"github.com/fromforgesoftware/go-kit/persistence"
)

// Querier interface that both *sql.DB and *sql.Tx implement
// This allows repositories to work with either the main DB or a transaction
type Querier interface {
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

// txKey is the context key for storing SQL transactions
type txKey struct{}

// InjectTx injects SQL transaction into context
// If transaction already exists in context, returns context as-is (allows nested transactions)
func InjectTx(ctx context.Context, tx *sql.Tx) context.Context {
	if sqlTxExists(ctx) {
		return ctx // Already in transaction - reuse it
	}
	return context.WithValue(ctx, txKey{}, tx)
}

// sqlTxExists checks if a transaction exists in the context
func sqlTxExists(ctx context.Context) bool {
	return extractTx(ctx) != nil
}

// extractTx extracts SQL transaction from context
func extractTx(ctx context.Context) *sql.Tx {
	if tx, ok := ctx.Value(txKey{}).(*sql.Tx); ok {
		return tx
	}
	return nil
}

// GetTx returns the appropriate querier from context
// Returns transaction if in txCtx, otherwise returns the regular DB
// This is the key helper that repositories use to automatically participate in transactions
func GetTx(ctx context.Context, db *sql.DB) Querier {
	if tx := extractTx(ctx); tx != nil {
		return tx // Use transaction
	}
	return db // Use regular DB
}

// transactioner implements persistence.Transactioner for SQL
type transactioner struct {
	db *sql.DB
}

// NewTransactioner creates a new SQL transactioner
func NewTransactioner(db *sql.DB) persistence.Transactioner {
	return &transactioner{db: db}
}

// Exec runs the given function within a transaction
// Automatically commits on success, rolls back on error
// Supports nested transactions by reusing existing transaction in context
func (t *transactioner) Exec(ctx context.Context, fn persistence.TxFunc) error {
	// If already in transaction, reuse it (nested transaction support)
	if sqlTxExists(ctx) {
		return fn(ctx)
	}

	// Begin new transaction
	tx, err := t.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	// Execute function with transaction context
	err = fn(InjectTx(ctx, tx))

	if err != nil {
		// Rollback on error — original error has precedence over a
		// rollback failure, which usually means the connection is dead.
		_ = tx.Rollback()
		return err
	}

	// Commit on success
	return tx.Commit()
}
