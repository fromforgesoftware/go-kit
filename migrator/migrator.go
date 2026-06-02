package migrator

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/fromforgesoftware/go-kit/monitoring/logger"
	"github.com/fromforgesoftware/go-kit/persistence/sqldb"
)

// migrator handles database migrations with pre/post script support.
type migrator struct {
	db          *sqldb.DBClient
	logger      logger.Logger
	serviceName string
}

// option configures a migrator.
type option func(*migrator)

// WithLogger sets a custom logger.
func WithLogger(log logger.Logger) option {
	return func(m *migrator) {
		m.logger = log
	}
}

// WithServiceName sets the service name for the migration table.
func WithServiceName(name string) option {
	return func(m *migrator) {
		m.serviceName = name
	}
}

// defaultOptions returns the default options for a migrator.
func defaultOptions() []option {
	return []option{
		WithLogger(logger.New()),
		WithServiceName("default"),
	}
}

// New creates a new migrator with the given database and options.
// DB parameter is required for explicit dependency management.
func New(db *sqldb.DBClient, opts ...option) (*migrator, error) {
	if db == nil {
		return nil, fmt.Errorf("db is required")
	}

	m := &migrator{
		db: db,
	}

	// Apply default options first, then user options
	for _, opt := range append(defaultOptions(), opts...) {
		opt(m)
	}

	return m, nil
}

// Run executes the migrations with pre/post scripts.
// Scripts always execute if present in migrationsFS.
// Expects migrationsFS structure:
//   - migrations/*.sql (actual migrations)
//   - migrations/common-pre-migration/*.sql (optional)
//   - migrations/common-post-migration/*.sql (optional)
func (m *migrator) Run(ctx context.Context, migrationsFS fs.FS) error {
	m.logger.WithKeysAndValues("service", m.serviceName).Info("🚀 Starting migration process")

	// Execute pre-migration scripts (if present)
	if err := m.executeScripts(ctx, migrationsFS, "migrations/common-pre-migration", "pre-migration"); err != nil {
		return fmt.Errorf("pre-migration scripts failed: %w", err)
	}

	// Run actual migrations
	if err := m.runMigrations(ctx, migrationsFS); err != nil {
		return fmt.Errorf("migrations failed: %w", err)
	}

	// Execute post-migration scripts (if present)
	if err := m.executeScripts(ctx, migrationsFS, "migrations/common-post-migration", "post-migration"); err != nil {
		return fmt.Errorf("post-migration scripts failed: %w", err)
	}

	m.logger.WithKeysAndValues("service", m.serviceName).Info("🎉 Migration completed successfully")
	return nil
}

// runMigrations executes the actual database migrations using golang-migrate.
func (m *migrator) runMigrations(ctx context.Context, migrationsFS fs.FS) error {
	m.logger.Info("📦 Running database migrations...")

	// Create source from embedded filesystem
	d, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("failed to read migrations folder: %w", err)
	}

	// Build DSN with service-specific migration table
	dsn, err := sqldb.NewDSN(sqldb.DriverTypePostgres)
	if err != nil {
		return fmt.Errorf("failed to create DSN: %w", err)
	}

	// Add migration table parameter
	serviceDSN := fmt.Sprintf("%s&x-migrations-table=%s_schema_migrations", dsn, m.serviceName)

	// Create migrate instance
	migrator, err := migrate.NewWithSourceInstance("iofs", d, serviceDSN)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer func() {
		if srcErr, dbErr := migrator.Close(); srcErr != nil || dbErr != nil {
			m.logger.WithKeysAndValues("source_err", srcErr, "db_err", dbErr).Warn("⚠️  Failed to close migrate instance")
		}
	}()

	// Get current version
	currentVersion, dirty, err := migrator.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		m.logger.WithKeysAndValues("error", err).Warn("⚠️  Could not determine current version")
	} else if errors.Is(err, migrate.ErrNilVersion) {
		m.logger.Info("📊 No migrations applied yet")
	} else {
		m.logger.WithKeysAndValues("version", currentVersion, "dirty", dirty).Info("📊 Current version")
	}

	// Run migration up
	if err := migrator.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			m.logger.Info("✅ No new migrations to apply")
			return nil
		}
		return fmt.Errorf("migration up failed: %w", err)
	}

	// Get final version
	finalVersion, finalDirty, err := migrator.Version()
	if err == nil {
		m.logger.WithKeysAndValues("version", finalVersion, "dirty", finalDirty).Info("📊 Final version")
	}

	m.logger.Info("✅ Migrations applied successfully")
	return nil
}

// executeScripts executes SQL scripts from a directory.
// Silently skips if directory doesn't exist (scripts are optional).
func (m *migrator) executeScripts(ctx context.Context, migrationsFS fs.FS, scriptPath, scriptType string) error {
	// Check if directory exists
	entries, err := fs.ReadDir(migrationsFS, scriptPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			m.logger.WithKeysAndValues("type", scriptType).Info("⏭️  No scripts found (optional)")
			return nil
		}
		return fmt.Errorf("failed to read %s directory: %w", scriptType, err)
	}

	// Collect SQL files
	var sqlFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".sql") {
			sqlFiles = append(sqlFiles, entry.Name())
		}
	}

	if len(sqlFiles) == 0 {
		m.logger.WithKeysAndValues("type", scriptType).Info("⏭️  No scripts to execute")
		return nil
	}

	// Sort for deterministic execution
	sort.Strings(sqlFiles)

	m.logger.WithKeysAndValues("count", len(sqlFiles), "type", scriptType).Info("🔄 Executing scripts")

	// Execute each script
	for _, filename := range sqlFiles {
		m.logger.WithKeysAndValues("type", scriptType, "filename", filename).Info("🔄 Executing script")

		content, err := fs.ReadFile(migrationsFS, filepath.Join(scriptPath, filename))
		if err != nil {
			return fmt.Errorf("failed to read script %s: %w", filename, err)
		}

		if _, err := m.db.Exec(ctx, string(content)); err != nil {
			return fmt.Errorf("failed to execute script %s: %w", filename, err)
		}

		m.logger.WithKeysAndValues("filename", filename).Info("✅ Executed")
	}

	m.logger.WithKeysAndValues("type", scriptType).Info("✅ Scripts completed")
	return nil
}

// Up is a convenience function that creates DB from environment and runs migrations.
func Up(ctx context.Context, migrationsFS fs.FS, opts ...option) error {
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

	m, err := New(dbClient, opts...)
	if err != nil {
		return err
	}

	return m.Run(ctx, migrationsFS)
}
