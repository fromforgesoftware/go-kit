// Package gormlitetest hands a test a SQLite database of its own: a file under t.TempDir(),
// optionally migrated with the kit migrator, closed when the test ends. Nothing external is
// needed, so these tests belong in the default suite.
package gormlitetest

import (
	"context"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/fromforgesoftware/go-kit/migrator"
	"github.com/fromforgesoftware/go-kit/monitoring/monitoringtest"
	"github.com/fromforgesoftware/go-kit/persistence/gormdb"
	"github.com/fromforgesoftware/go-kit/persistence/gormdb/gormlite"
	"github.com/fromforgesoftware/go-kit/persistence/sqldb"
)

const fileName = "test.db"

type (
	// TestDB is the database handed to a test, with the path of the file behind it.
	TestDB struct {
		*gormdb.DBClient
		Path string
	}

	// Option configures GetDB.
	Option func(*config)

	config struct {
		migrations fs.FS
		service    string
	}
)

// WithMigrations applies the migrations in fsys (the kit migrator's layout) under the given
// service name before the database is handed out.
func WithMigrations(fsys fs.FS, service string) Option {
	return func(c *config) {
		c.migrations = fsys
		c.service = service
	}
}

// GetDB opens a fresh database file for t and closes it when t ends.
func GetDB(t *testing.T, opts ...Option) *TestDB {
	t.Helper()

	cfg := &config{service: "test"}
	for _, opt := range opts {
		opt(cfg)
	}

	path := filepath.Join(t.TempDir(), fileName)
	client, err := gormlite.NewClient(path, monitoringtest.NewMonitor(t))
	if err != nil {
		t.Fatalf("open sqlite test database: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	if cfg.migrations != nil {
		conn, err := client.Database()
		if err != nil {
			t.Fatalf("sqlite test database handle: %v", err)
		}
		m, err := migrator.New(sqldb.NewDBClient(conn),
			migrator.WithServiceName(cfg.service),
			migrator.WithDriver(sqldb.DriverTypeSQLite),
		)
		if err != nil {
			t.Fatalf("build migrator: %v", err)
		}
		if err := m.Run(context.Background(), cfg.migrations); err != nil {
			t.Fatalf("migrate sqlite test database: %v", err)
		}
	}

	return &TestDB{DBClient: client, Path: path}
}
