package gormlite

import (
	"net/url"
	"strconv"
	"time"

	"gorm.io/driver/sqlite"
	_ "modernc.org/sqlite" // registers the "sqlite" database/sql driver

	"github.com/fromforgesoftware/go-kit/monitoring"
	"github.com/fromforgesoftware/go-kit/persistence/gormdb"
	"github.com/fromforgesoftware/go-kit/persistence/sqldb"
)

const (
	driverName   = "sqlite"
	maxOpenConns = 4
	maxIdleConns = 2
	busyTimeout  = 5 * time.Second
)

// DSN builds the connection string for a database file: WAL journaling, synchronous=FULL, foreign
// keys enforced, a busy timeout, and times stored in SQLite's own text format.
func DSN(path string) string {
	q := url.Values{}
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "synchronous(FULL)")
	q.Add("_pragma", "foreign_keys(ON)")
	q.Add("_pragma", "busy_timeout("+strconv.FormatInt(busyTimeout.Milliseconds(), 10)+")")
	q.Set("_time_format", "sqlite")
	q.Set("_txlock", "immediate")
	return "file:" + path + "?" + q.Encode()
}

// NewClient opens the database file at path. The file and its directory must exist or be
// creatable by the process. Options extend gormdb's defaults.
func NewClient(path string, m monitoring.Monitor, options ...gormdb.Option) (*gormdb.DBClient, error) {
	if path == "" {
		return nil, sqldb.NewErrEmptyDBConnection()
	}
	dialector := sqlite.New(sqlite.Config{DriverName: driverName, DSN: DSN(path)})
	options = append([]gormdb.Option{
		gormdb.WithoutDefaultConnectionOptions(),
		gormdb.WithSQLConnectionOptions(
			sqldb.WithMaxOpenLimit(maxOpenConns),
			sqldb.WithMaxIdleConns(maxIdleConns),
		),
	}, options...)
	return gormdb.New(dialector, m, options...)
}
