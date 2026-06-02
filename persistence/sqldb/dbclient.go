package sqldb

import (
	"context"
	"database/sql"
)

// DBClient wraps a *sql.DB and provides additional functionality
type DBClient struct {
	db *sql.DB
}

// NewDBClient creates a new DBClient from a *sql.DB
func NewDBClient(db *sql.DB) *DBClient {
	return &DBClient{db: db}
}

// DB returns the underlying *sql.DB
func (c *DBClient) DB() *sql.DB {
	return c.db
}

// Exec executes a query without returning rows
func (c *DBClient) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return c.db.ExecContext(ctx, query, args...)
}

// Query executes a query that returns rows
func (c *DBClient) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return c.db.QueryContext(ctx, query, args...)
}

// QueryRow executes a query that returns at most one row
func (c *DBClient) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return c.db.QueryRowContext(ctx, query, args...)
}

// Begin starts a new transaction with default options.
func (c *DBClient) Begin(ctx context.Context) (*sql.Tx, error) {
	return c.db.BeginTx(ctx, nil)
}

// BeginTx starts a new transaction with the given options.
func (c *DBClient) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return c.db.BeginTx(ctx, opts)
}

// Close closes the database connection.
func (c *DBClient) Close() error {
	return c.db.Close()
}

// Ping verifies the connection is alive
func (c *DBClient) Ping(ctx context.Context) error {
	return c.db.PingContext(ctx)
}
