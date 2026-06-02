//go:build integration
// +build integration

package sqldbtest

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/orlangure/gnomock"
	"github.com/orlangure/gnomock/preset/postgres"

	"github.com/fromforgesoftware/go-kit/persistence/sqldb"
)

type SchemaName string

func (s SchemaName) String() string { return string(s) }

const (
	TestSchema SchemaName = "test"
)

const (
	testDBUser     = "gnomock"
	testDBPassword = "gnomick"
	testDBName     = "test"
)

var (
	databases = make(map[string]*testDB)
	mu        sync.Mutex
)

type testDBConfig struct {
	schemaMigrationFolderPath string
	migrationFolderPath       string
}

type TestDBConfigOption func(*testDBConfig)

func TestDBConfigWithSchemaMigrationsFolderPath(path string) TestDBConfigOption {
	return func(td *testDBConfig) {
		td.schemaMigrationFolderPath = resolvePath(path)
	}
}

func TestDBConfigWithMigrationsFolderPath(path string) TestDBConfigOption {
	return func(td *testDBConfig) { td.migrationFolderPath = resolvePath(path) }
}

func testDBConfigDefaultOptions() []TestDBConfigOption {
	return []TestDBConfigOption{
		TestDBConfigWithSchemaMigrationsFolderPath(getSchemaMigrationFolder()),
	}
}

type testDB struct {
	*sqldb.DBClient
	Host     string
	Port     int
	Schema   string
	DBName   string
	User     string
	Password string
}

// GetDB creates or returns a singleton test database for the given schema with migrations applied.
// The database is created once per schema per test run and reused.
func GetDB(t *testing.T, schema SchemaName, opts ...TestDBConfigOption) *testDB {
	t.Helper()

	mu.Lock()
	defer mu.Unlock()

	if db, ok := databases[schema.String()]; ok {
		return db
	}

	cfg := &testDBConfig{}
	for _, opt := range append(testDBConfigDefaultOptions(), opts...) {
		opt(cfg)
	}

	db := createPGSQLContainer(t, schema, cfg)
	if db != nil {
		databases[schema.String()] = db
	}
	return db
}

func createPGSQLContainer(t *testing.T, schema SchemaName, cfg *testDBConfig) *testDB {
	t.Helper()

	extensionSetup := `CREATE EXTENSION IF NOT EXISTS "uuid-ossp";`

	options := []postgres.Option{
		postgres.WithQueries(extensionSetup), // Runs before migrations
		postgres.WithUser(testDBUser, testDBPassword),
		postgres.WithDatabase(testDBName),
		postgres.WithVersion("14.6-alpine"),
		postgres.WithTimezone(time.UTC.String()),
	}

	if cfg.migrationFolderPath != "" {
		migrationOpts := getMigrationsOptions(t, schema, cfg.schemaMigrationFolderPath, cfg.migrationFolderPath)
		options = append(options, migrationOpts...)
	}

	p := postgres.Preset(options...)
	container, err := gnomock.Start(p)
	if err != nil {
		t.Logf("Unable to start gnomock container (skipping test): %v", err)
		return nil
	}

	// Use sqldb.NewDSN to construct DSN properly
	dsnURL, err := sqldb.NewDSN(
		sqldb.DriverTypePostgres,
		sqldb.WithConnHost(container.Host),
		sqldb.WithConnPort(fmt.Sprintf("%d", container.DefaultPort())),
		sqldb.WithConnUser(testDBUser),
		sqldb.WithConnPwd(testDBPassword),
		sqldb.WithConnDBName(testDBName),
		sqldb.WithConnSSLMode("disable"),
		sqldb.WithConnSearchPath(fmt.Sprintf("%s,public", schema.String())),
	)
	if err != nil {
		t.Logf("Unable to create DSN (skipping test): %v", err)
		return nil
	}

	// Use sqldb.Connect instead of sql.Open directly
	db, err := sqldb.Connect(dsnURL)
	if err != nil {
		t.Logf("Unable to connect to test database (skipping test): %v", err)
		return nil
	}

	if err := db.Ping(); err != nil {
		t.Logf("Unable to ping test database (skipping test): %v", err)
		return nil
	}

	client := sqldb.NewDBClient(db)

	return &testDB{
		DBClient: client,
		Host:     container.Host,
		Port:     container.DefaultPort(),
		Schema:   schema.String(),
		DBName:   testDBName,
		User:     testDBUser,
		Password: testDBPassword,
	}
}

func getMigrationsOptions(t *testing.T, schema SchemaName, schemaMigrationFolderPath, migrationFolderPath string) []postgres.Option {
	t.Helper()

	opts := []postgres.Option{}

	// 1. Pre-migrations
	preMigrationPath := filepath.Join(migrationFolderPath, "common-pre-migration", "*.sql")
	preMatches, _ := filepath.Glob(preMigrationPath)
	sort.Strings(preMatches)
	for _, m := range preMatches {
		t.Logf("Adding pre-migration: %s", filepath.Base(m))
		opts = append(opts, postgres.WithQueriesFile(m))
	}

	// 2. Schema creation
	if schemaMigrationFolderPath != "" {
		schemaFile := filepath.Join(schemaMigrationFolderPath, fmt.Sprintf("%s.sql", schema.String()))
		t.Logf("Adding schema file: %s", schemaFile)
		opts = append(opts, postgres.WithQueriesFile(schemaFile))
	}

	// 3. Main migrations
	pattern := filepath.Join(migrationFolderPath, "*.up.sql")
	matches, _ := filepath.Glob(pattern)
	sort.Strings(matches)
	for _, m := range matches {
		if strings.Contains(m, "analytics") {
			continue
		}
		t.Logf("Adding migration: %s", filepath.Base(m))
		opts = append(opts, postgres.WithQueriesFile(m))
	}

	// 4. Post-migrations
	postMigrationPath := filepath.Join(migrationFolderPath, "common-post-migration", "*.sql")
	postMatches, _ := filepath.Glob(postMigrationPath)
	sort.Strings(postMatches)
	for _, m := range postMatches {
		t.Logf("Adding post-migration: %s", filepath.Base(m))
		opts = append(opts, postgres.WithQueriesFile(m))
	}

	return opts
}

func getSchemaMigrationFolder() string {
	_, filename, _, _ := runtime.Caller(0)
	// Default schema files live alongside pgtest migrations
	return filepath.Join(filepath.Dir(filename), "migrations")
}

// resolvePath resolves a relative path to an absolute path, correctly handling:
// 1. Bazel runfiles (via TEST_SRCDIR)
// 2. Local execution (via absolute path resolution)
func resolvePath(relPath string) string {
	// Bazel sets TEST_SRCDIR to the runfiles directory
	if srcDir := os.Getenv("TEST_SRCDIR"); srcDir != "" {
		workspace := os.Getenv("TEST_WORKSPACE")
		if workspace == "" {
			workspace = "_main"
		}
		return filepath.Join(srcDir, workspace, relPath)
	}
	// Local runs use absolute paths
	if absPath, err := filepath.Abs(relPath); err == nil {
		return absPath
	}
	return relPath
}
