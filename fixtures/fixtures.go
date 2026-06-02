package fixtures

import (
	"context"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strings"

	"github.com/fromforgesoftware/go-kit/monitoring/logger"
	"github.com/fromforgesoftware/go-kit/persistence/sqldb"
)

// Flexible pattern: any number of digits followed by underscore
// Supports: 0001_init.sql, 001_init.sql, 20250123000000_init.sql, etc.
var filenamePattern = regexp.MustCompile(`^(\d+)_.*\.sql$`)

// loader handles loading SQL fixtures into a database.
type loader struct {
	db     *sqldb.DBClient
	logger logger.Logger
}

// option configures a loader.
type option func(*loader)

// WithLogger sets a custom logger.
func WithLogger(log logger.Logger) option {
	return func(l *loader) {
		l.logger = log
	}
}

// defaultOptions returns the default options for a loader.
func defaultOptions() []option {
	return []option{
		WithLogger(logger.New()),
	}
}

// New creates a new loader with the given database and options.
// DB parameter is required for explicit dependency management.
func New(db *sqldb.DBClient, opts ...option) (*loader, error) {
	if db == nil {
		return nil, fmt.Errorf("db is required")
	}

	l := &loader{
		db: db,
	}

	// Apply default options first, then user options
	for _, opt := range append(defaultOptions(), opts...) {
		opt(l)
	}

	return l, nil
}

// Load loads SQL fixtures from the embedded filesystem.
func (l *loader) Load(ctx context.Context, fixturesFS fs.FS) error {
	if err := l.db.DB().PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	files, err := readSQLFiles(fixturesFS)
	if err != nil {
		return err
	}

	if len(files) == 0 {
		l.logger.Info("⚠️  No SQL fixture files found")
		return nil
	}

	l.logger.WithKeysAndValues("count", len(files)).Info("📁 Loading fixture files")

	// Use Transactioner for automatic commit/rollback
	transactioner := sqldb.NewTransactioner(l.db.DB())

	return transactioner.Exec(ctx, func(ctx context.Context) error {
		// Get the transaction from context
		tx := sqldb.GetTx(ctx, l.db.DB())

		for _, filename := range files {
			l.logger.WithKeysAndValues("filename", filename).Info("🔄 Executing fixture")

			content, err := fs.ReadFile(fixturesFS, filename)
			if err != nil {
				return fmt.Errorf("failed to read %s: %w", filename, err)
			}

			if _, err := tx.ExecContext(ctx, string(content)); err != nil {
				return fmt.Errorf("failed to execute %s: %w", filename, err)
			}

			l.logger.WithKeysAndValues("filename", filename).Info("✅ Executed fixture")
		}

		l.logger.WithKeysAndValues("count", len(files)).Info("🎉 Successfully loaded fixtures")
		return nil // Auto-commits on success
	})
}

// Load is a convenience function that creates DB from environment and loads fixtures.
func Load(ctx context.Context, fixturesFS fs.FS) error {
	// Create DB from environment variables
	dsn, err := sqldb.NewDSN(sqldb.DriverTypePostgres)
	if err != nil {
		return fmt.Errorf("failed to generate DSN: %w", err)
	}

	db, err := sqldb.Connect(dsn)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer func() { _ = db.Close() }()

	sqldb.ConfigureDefaultPool(db)
	dbClient := sqldb.NewDBClient(db)

	loader, err := New(dbClient)
	if err != nil {
		return err
	}

	return loader.Load(ctx, fixturesFS)
}

// readSQLFiles reads and validates SQL files.
func readSQLFiles(fixturesFS fs.FS) ([]string, error) {
	entries, err := fs.ReadDir(fixturesFS, ".")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded filesystem: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}

		if !filenamePattern.MatchString(name) {
			return nil, fmt.Errorf("invalid fixture filename '%s': must match pattern <digits>_name.sql (e.g., 0001_init.sql)", name)
		}

		files = append(files, name)
	}

	sort.Strings(files)
	return files, nil
}
