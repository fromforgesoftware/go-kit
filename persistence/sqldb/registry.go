package sqldb

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
)

// StatementID uniquely identifies a prepared statement
type StatementID string

// Registry manages prepared statements for performance
type Registry struct {
	db    *sql.DB
	mu    sync.RWMutex
	stmts map[StatementID]*sql.Stmt
}

// NewRegistry creates a new statement registry
func NewRegistry(db *sql.DB) *Registry {
	return &Registry{
		db:    db,
		stmts: make(map[StatementID]*sql.Stmt),
	}
}

// Register prepares and caches a statement
func (r *Registry) Register(ctx context.Context, id StatementID, query string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if already registered
	if _, exists := r.stmts[id]; exists {
		return fmt.Errorf("statement %s already registered", id)
	}

	stmt, err := r.db.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to prepare statement %s: %w", id, err)
	}

	r.stmts[id] = stmt
	return nil
}

// MustRegister registers a statement and panics on error (useful for init)
func (r *Registry) MustRegister(ctx context.Context, id StatementID, query string) {
	if err := r.Register(ctx, id, query); err != nil {
		panic(err)
	}
}

// Get retrieves a prepared statement by ID
func (r *Registry) Get(id StatementID) (*sql.Stmt, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stmt, exists := r.stmts[id]
	if !exists {
		return nil, fmt.Errorf("statement %s not registered", id)
	}

	return stmt, nil
}

// MustGet retrieves a statement and panics if not found
func (r *Registry) MustGet(id StatementID) *sql.Stmt {
	stmt, err := r.Get(id)
	if err != nil {
		panic(err)
	}
	return stmt
}

// Close closes all prepared statements
func (r *Registry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var errs []error
	for id, stmt := range r.stmts {
		if err := stmt.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close statement %s: %w", id, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing statements: %v", errs)
	}

	return nil
}

// Count returns the number of registered statements
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.stmts)
}
