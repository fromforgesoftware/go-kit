package sqldb

import (
	"database/sql"
	"time"
)

// PoolOption defines a functional option for configuring connection pool
type PoolOption func(*sql.DB)

// WithPoolMaxOpenConns sets the maximum number of open connections to the database.
// If n <= 0, then there is no limit on the number of open connections.
func WithPoolMaxOpenConns(n int) PoolOption {
	return func(db *sql.DB) {
		if n > 0 {
			db.SetMaxOpenConns(n)
		}
	}
}

// WithPoolMaxIdleConns sets the maximum number of connections in the idle connection pool.
// If n <= 0, no idle connections are retained.
func WithPoolMaxIdleConns(n int) PoolOption {
	return func(db *sql.DB) {
		if n >= 0 {
			db.SetMaxIdleConns(n)
		}
	}
}

// WithPoolConnMaxLifetime sets the maximum amount of time a connection may be reused.
// Expired connections may be closed lazily before reuse.
// If d <= 0, connections are not closed due to a connection's age.
func WithPoolConnMaxLifetime(d time.Duration) PoolOption {
	return func(db *sql.DB) {
		if d > 0 {
			db.SetConnMaxLifetime(d)
		}
	}
}

// WithPoolConnMaxIdleTime sets the maximum amount of time a connection may be idle.
// Expired connections may be closed lazily before reuse.
// If d <= 0, connections are not closed due to a connection's idle time.
func WithPoolConnMaxIdleTime(d time.Duration) PoolOption {
	return func(db *sql.DB) {
		if d > 0 {
			db.SetConnMaxIdleTime(d)
		}
	}
}

// DefaultPoolOptions returns recommended defaults for production
func DefaultPoolOptions() []PoolOption {
	return []PoolOption{
		WithPoolMaxOpenConns(25),                  // Reasonable default for most applications
		WithPoolMaxIdleConns(5),                   // Keep some connections warm
		WithPoolConnMaxLifetime(5 * time.Minute),  // Refresh connections periodically
		WithPoolConnMaxIdleTime(10 * time.Minute), // Close idle connections after 10min
	}
}

// HighConcurrencyPoolOptions returns settings optimized for high-concurrency workloads (MMO servers)
func HighConcurrencyPoolOptions() []PoolOption {
	return []PoolOption{
		WithPoolMaxOpenConns(100),                // Support many concurrent queries
		WithPoolMaxIdleConns(20),                 // Keep more connections warm
		WithPoolConnMaxLifetime(3 * time.Minute), // Refresh more frequently
		WithPoolConnMaxIdleTime(5 * time.Minute), // Close idle faster to free resources
	}
}

// ConfigurePool applies pool options to a database connection
func ConfigurePool(db *sql.DB, opts ...PoolOption) {
	for _, opt := range opts {
		opt(db)
	}
}

// ConfigureDefaultPool applies default pool configuration
func ConfigureDefaultPool(db *sql.DB) {
	ConfigurePool(db, DefaultPoolOptions()...)
}

// ConfigureHighConcurrencyPool applies high-concurrency pool configuration
func ConfigureHighConcurrencyPool(db *sql.DB) {
	ConfigurePool(db, HighConcurrencyPoolOptions()...)
}
