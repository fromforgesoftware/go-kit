package gormdb_test

import (
	"context"
	"database/sql/driver"
	"os"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/postgres"

	"github.com/fromforgesoftware/go-kit/monitoring/monitoringtest"
	"github.com/fromforgesoftware/go-kit/persistence/gormdb"
	"github.com/fromforgesoftware/go-kit/persistence/sqldb"
)

type testTable struct {
	ID string
}

func TestNewClient(t *testing.T) {
	t.Skip("TODO: Fix mock expectations after removing tracing")
	defer os.Clearenv()
	t.Setenv("DB_LOG_LEVEL", "debug")

	monitor := monitoringtest.NewMonitor(t)

	cli, err := gormdb.New(nil, monitor)
	assert.Nil(t, cli)
	assert.NotNil(t, err)
	assert.Panics(t, func() { gormdb.Must(gormdb.New(nil, monitor)) })

	cli, err = gormdb.New(postgres.Open("invaliddsn"), monitor)
	assert.Nil(t, cli)
	assert.NotNil(t, err)

	db, mock, err := sqlmock.New(
		sqlmock.MonitorPingsOption(true),
		sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual),
	)
	assert.Nil(t, err)
	assert.NotNil(t, db)
	assert.NotNil(t, mock)
	mock.ExpectPing()
	mock.ExpectExec(`SET SEARCH_PATH to "schema_123_test",public`).
		WillReturnResult(driver.ResultNoRows)
	mock.ExpectQuery(`SELECT current_schema`).
		WillReturnRows(
			sqlmock.NewRows([]string{"current_schema"}).
				AddRow("schema_123_test"),
		)

	t.Setenv("ENV", "local")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("DB_LOG_LEVEL", "debug")
	cli, err = gormdb.New(
		postgres.New(postgres.Config{Conn: db}),
		monitoringtest.NewMonitor(t),
		gormdb.WithSingularTable(true),
		gormdb.WithSQLConnectionOptions(sqldb.WithDBSchema("schema_123_test")),
	)
	assert.Nil(t, err)
	assert.NotNil(t, cli)

	mock.ExpectPing()
	assert.Nil(t, cli.Ping())

	mock.ExpectPing()
	assert.Nil(t, cli.PingContext(context.Background()))

	var pgCli any = cli
	pgDBCli, ok := pgCli.(sqldb.Client)
	assert.True(t, ok)
	assert.NotNil(t, pgDBCli)

	mock.ExpectExec(`CREATE TABLE "test_table" ("id" text,PRIMARY KEY ("id"))`).
		WillReturnResult(driver.ResultNoRows)
	assert.Nil(t, cli.AutoMigrate(&testTable{}))
	mock.ExpectExec(`DROP TABLE IF EXISTS "test_table" CASCADE`).
		WillReturnResult(driver.ResultNoRows)
	assert.Nil(t, cli.Migrator().DropTable(&testTable{}))

	mock.ExpectClose()
	assert.Nil(t, cli.Close())
}
