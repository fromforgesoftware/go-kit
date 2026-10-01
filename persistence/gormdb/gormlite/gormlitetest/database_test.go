package gormlitetest_test

import (
	"context"
	"embed"
	"io/fs"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/migrator"
	"github.com/fromforgesoftware/go-kit/persistence/gormdb/gormlite/gormlitetest"
	"github.com/fromforgesoftware/go-kit/persistence/sqldb"
)

//go:embed testdata
var testdata embed.FS

func TestGetDBAppliesTheMigrations(t *testing.T) {
	t.Parallel()
	db := gormlitetest.GetDB(t, gormlitetest.WithMigrations(migrations(t), "users"))

	var usernames []string
	require.NoError(t, db.Table("users").Order("username").Pluck("username", &usernames).Error)
	assert.Equal(t, []string{"alice", "bob"}, usernames)

	var version int
	var dirty bool
	require.NoError(t, db.Raw("SELECT version, dirty FROM users_schema_migrations").Row().Scan(&version, &dirty))
	assert.Equal(t, 2, version)
	assert.False(t, dirty)
}

func TestMigratingTwiceIsANoOp(t *testing.T) {
	t.Parallel()
	db := gormlitetest.GetDB(t, gormlitetest.WithMigrations(migrations(t), "users"))
	conn, err := db.Database()
	require.NoError(t, err)
	m, err := migrator.New(sqldb.NewDBClient(conn),
		migrator.WithServiceName("users"), migrator.WithDriver(sqldb.DriverTypeSQLite))
	require.NoError(t, err)

	require.NoError(t, m.Run(context.Background(), migrations(t)))

	var count int64
	require.NoError(t, db.Table("users").Count(&count).Error)
	assert.Equal(t, int64(2), count)
}

func TestEachTestGetsItsOwnFile(t *testing.T) {
	t.Parallel()
	first := gormlitetest.GetDB(t)
	second := gormlitetest.GetDB(t)

	assert.NotEqual(t, first.Path, second.Path)
	assert.FileExists(t, first.Path)
	assert.FileExists(t, second.Path)
}

func TestMigratorRefusesAnUnknownDriver(t *testing.T) {
	t.Parallel()
	db := gormlitetest.GetDB(t)
	conn, err := db.Database()
	require.NoError(t, err)
	m, err := migrator.New(sqldb.NewDBClient(conn), migrator.WithDriver("oracle"))
	require.NoError(t, err)

	err = m.Run(context.Background(), migrations(t))

	require.ErrorContains(t, err, `unsupported migration driver "oracle"`)
}

func migrations(t *testing.T) fs.FS {
	t.Helper()
	sub, err := fs.Sub(testdata, "testdata")
	require.NoError(t, err)
	return sub
}
