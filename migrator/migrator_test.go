//go:build integration
// +build integration

package migrator_test

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"reflect"
	"testing"

	"github.com/fromforgesoftware/go-kit/migrator"
	"github.com/fromforgesoftware/go-kit/monitoring/logger"
	"github.com/fromforgesoftware/go-kit/persistence/sqldb"
	"github.com/fromforgesoftware/go-kit/persistence/sqldb/sqldbtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed testdata
var testMigrationsFS embed.FS

// setupTestEnv sets up environment variables from the test database for the migrator to use.
func setupTestEnv(t *testing.T, db any) {
	t.Helper()
	// Access unexported testDB fields using reflection
	v := reflect.ValueOf(db).Elem()
	t.Setenv("DB_HOST", v.FieldByName("Host").String())
	t.Setenv("DB_PORT", fmt.Sprintf("%d", v.FieldByName("Port").Int()))
	t.Setenv("DB_USER", v.FieldByName("User").String())
	t.Setenv("DB_PASSWORD", v.FieldByName("Password").String())
	t.Setenv("DB_NAME", v.FieldByName("DBName").String())
	t.Setenv("DB_SSL", "disable")
	t.Setenv("DB_SEARCH_PATH", v.FieldByName("Schema").String())
}

// TestNewRequiresDB tests that New() requires a DB parameter.
func TestNewRequiresDB(t *testing.T) {
	_, err := migrator.New(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "db is required")
}

// TestNewWithDB tests creating a migrator with a database.
func TestNewWithDB(t *testing.T) {
	testDB := sqldbtest.GetDB(t, sqldbtest.TestSchema)

	m, err := migrator.New(
		sqldb.NewDBClient(testDB.DB()),
		migrator.WithServiceName("test"),
		migrator.WithLogger(logger.New()),
	)
	require.NoError(t, err)
	require.NotNil(t, m)
}

// TestMigrationIntegration tests the full migration workflow with pre/post scripts.
func TestMigrationIntegration(t *testing.T) {
	ctx := context.Background()

	// Get test database with test schema set as search_path
	testDB := sqldbtest.GetDB(t, sqldbtest.TestSchema)

	// Set environment variables for the migrator to use
	setupTestEnv(t, testDB)

	// Wrap embed.FS to read from testdata subdirectory
	subFS, err := fs.Sub(testMigrationsFS, "testdata")
	require.NoError(t, err, "Failed to get testdata subdirectory")

	// Create migrator with test database
	m, err := migrator.New(
		sqldb.NewDBClient(testDB.DB()),
		migrator.WithServiceName("test"),
		migrator.WithLogger(logger.New()),
	)
	require.NoError(t, err)

	// Run migrations
	err = m.Run(ctx, subFS)
	require.NoError(t, err)

	// Verify pre-migration script executed
	var preEvent string
	err = testDB.QueryRow(ctx, "SELECT event FROM migration_tracking WHERE event = 'pre-migration-executed'").Scan(&preEvent)
	require.NoError(t, err, "Pre-migration script should have created tracking table and inserted event")
	assert.Equal(t, "pre-migration-executed", preEvent)

	// Verify post-migration script executed
	var postEvent string
	err = testDB.QueryRow(ctx, "SELECT event FROM migration_tracking WHERE event = 'post-migration-executed'").Scan(&postEvent)
	require.NoError(t, err, "Post-migration script should have inserted event")
	assert.Equal(t, "post-migration-executed", postEvent)

	// Verify migration 1: table was created by trying to query it
	// If the table doesn't exist, this will fail
	var count int
	err = testDB.QueryRow(ctx, "SELECT COUNT(*) FROM migrator_test_users").Scan(&count)
	require.NoError(t, err, "Migration should have created migrator_test_users table")
	// Verify migration 2: data was inserted
	assert.Equal(t, 2, count, "Migration should have inserted 2 users")

	// Verify specific user data
	var username, email string
	err = testDB.QueryRow(ctx, "SELECT username, email FROM migrator_test_users WHERE username = $1", "testuser1").Scan(&username, &email)
	require.NoError(t, err)
	assert.Equal(t, "testuser1", username)
	assert.Equal(t, "test1@example.com", email)

	// Verify migration version table exists and check version
	var version int
	var dirty bool
	err = testDB.QueryRow(ctx, "SELECT version, dirty FROM test_schema_migrations").Scan(&version, &dirty)
	require.NoError(t, err, "Migration should have created version tracking table")
	assert.Equal(t, 2, version, "Should be at migration version 2")
	assert.False(t, dirty, "Migration should not be in dirty state")

}

// TestMigrationIdempotency verifies that running migrations multiple times doesn't cause errors.
func TestMigrationIdempotency(t *testing.T) {
	ctx := context.Background()

	testDB := sqldbtest.GetDB(t, sqldbtest.TestSchema)
	setupTestEnv(t, testDB)

	subFS, err := fs.Sub(testMigrationsFS, "testdata")
	require.NoError(t, err)

	m, err := migrator.New(
		sqldb.NewDBClient(testDB.DB()),
		migrator.WithServiceName("test"),
		migrator.WithLogger(logger.New()),
	)
	require.NoError(t, err)

	// Run migrations first time
	err = m.Run(ctx, subFS)
	require.NoError(t, err)

	// Verify initial state
	var count int
	err = testDB.QueryRow(ctx, "SELECT COUNT(*) FROM migrator_test_users").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	// Run migrations second time - should be no-op
	err = m.Run(ctx, subFS)
	require.NoError(t, err, "Running migrations again should not error")

	// Verify data hasn't changed
	err = testDB.QueryRow(ctx, "SELECT COUNT(*) FROM migrator_test_users").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 2, count, "Data should not be duplicated")

	// Verify version is still the same
	var version int
	err = testDB.QueryRow(ctx, "SELECT version FROM test_schema_migrations").Scan(&version)
	require.NoError(t, err)
	assert.Equal(t, 2, version)
}

// TestMigrationVersionTracking verifies migration version table creation and updates.
func TestMigrationVersionTracking(t *testing.T) {
	ctx := context.Background()

	testDB := sqldbtest.GetDB(t, sqldbtest.TestSchema)
	setupTestEnv(t, testDB)

	subFS, err := fs.Sub(testMigrationsFS, "testdata")
	require.NoError(t, err)

	// Create migrator with custom service name
	customServiceName := "custom_service"
	m, err := migrator.New(
		sqldb.NewDBClient(testDB.DB()),
		migrator.WithServiceName(customServiceName),
		migrator.WithLogger(logger.New()),
	)
	require.NoError(t, err)

	// Run migrations
	err = m.Run(ctx, subFS)
	require.NoError(t, err)

	// Verify custom migration table was created and tracks version
	var version int
	var dirty bool
	expectedTableName := customServiceName + "_schema_migrations"
	query := fmt.Sprintf("SELECT version, dirty FROM %s", expectedTableName)
	err = testDB.QueryRow(ctx, query).Scan(&version, &dirty)
	require.NoError(t, err, "Migration should have created service-specific version table")
	assert.Equal(t, 2, version, "Should be at version 2 after applying both migrations")
	assert.False(t, dirty, "Migration should complete cleanly")

}

// TestMigrationWithoutPrePostScripts tests migrations work when pre/post scripts are absent.
func TestMigrationWithoutPrePostScripts(t *testing.T) {
	ctx := context.Background()

	testDB := sqldbtest.GetDB(t, sqldbtest.TestSchema)
	setupTestEnv(t, testDB)

	// Create a filesystem that doesn't include pre/post migration scripts
	subFS, err := fs.Sub(testMigrationsFS, "testdata")
	require.NoError(t, err)

	// Wrap in a custom FS that blocks access to pre/post script directories
	migrationsOnlyFS := &migrationsOnlyWrapper{fs: subFS}

	m, err := migrator.New(
		sqldb.NewDBClient(testDB.DB()),
		migrator.WithServiceName("test_no_scripts"),
		migrator.WithLogger(logger.New()),
	)
	require.NoError(t, err)

	// Run migrations - should succeed even without pre/post scripts
	err = m.Run(ctx, migrationsOnlyFS)
	require.NoError(t, err)

	// Verify migrations ran successfully
	var count int
	err = testDB.QueryRow(ctx, "SELECT COUNT(*) FROM migrator_test_users").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	// Verify tracking table doesn't exist (since pre/post scripts didn't run)
	var trackingExists bool
	err = testDB.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'test' 
			AND table_name = 'migration_tracking'
		)
	`).Scan(&trackingExists)
	require.NoError(t, err)
	assert.False(t, trackingExists, "Tracking table should not exist without pre/post scripts")
}

// migrationsOnlyWrapper wraps an FS to return ErrNotExist for pre/post script directories.
type migrationsOnlyWrapper struct {
	fs fs.FS
}

func (m *migrationsOnlyWrapper) Open(name string) (fs.File, error) {
	// Block access to pre/post migration directories
	if name == "migrations/common-pre-migration" || name == "migrations/common-post-migration" {
		return nil, fs.ErrNotExist
	}
	return m.fs.Open(name)
}

func (m *migrationsOnlyWrapper) ReadDir(name string) ([]fs.DirEntry, error) {
	// Block access to pre/post migration directories
	if name == "migrations/common-pre-migration" || name == "migrations/common-post-migration" {
		return nil, fs.ErrNotExist
	}

	// For the migrations directory itself, filter out pre/post subdirectories
	if name == "migrations" || name == "." || name == "" {
		entries, err := fs.ReadDir(m.fs, name)
		if err != nil {
			return nil, err
		}

		var filtered []fs.DirEntry
		for _, entry := range entries {
			if entry.Name() != "common-pre-migration" && entry.Name() != "common-post-migration" {
				filtered = append(filtered, entry)
			}
		}
		return filtered, nil
	}

	return fs.ReadDir(m.fs, name)
}

func (m *migrationsOnlyWrapper) ReadFile(name string) ([]byte, error) {
	if rfs, ok := m.fs.(fs.ReadFileFS); ok {
		return rfs.ReadFile(name)
	}

	file, err := m.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	if stat, err := file.Stat(); err == nil {
		data := make([]byte, stat.Size())
		if _, err := file.Read(data); err != nil {
			return nil, err
		}
		return data, nil
	}

	return nil, fs.ErrInvalid
}

// TestInvalidMigrationFS tests error handling when migration filesystem is malformed.
func TestInvalidMigrationFS(t *testing.T) {
	ctx := context.Background()

	testDB := sqldbtest.GetDB(t, sqldbtest.TestSchema)

	m, err := migrator.New(
		sqldb.NewDBClient(testDB.DB()),
		migrator.WithServiceName("test_invalid"),
		migrator.WithLogger(logger.New()),
	)
	require.NoError(t, err)

	// Create an empty filesystem (no migrations directory)
	emptyFS := &emptyFS{}

	// Run migrations with invalid FS - should error
	err = m.Run(ctx, emptyFS)
	require.Error(t, err, "Should error with invalid filesystem")
	assert.Contains(t, err.Error(), "migrations failed", "Error should indicate migration failure")
}

// emptyFS is an empty filesystem for testing error handling.
type emptyFS struct{}

func (e *emptyFS) Open(name string) (fs.File, error) {
	return nil, fs.ErrNotExist
}

func (e *emptyFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if name == "migrations" {
		return nil, fs.ErrNotExist
	}
	return []fs.DirEntry{}, nil
}

func (e *emptyFS) ReadFile(name string) ([]byte, error) {
	return nil, fs.ErrNotExist
}
