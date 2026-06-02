package gormpg

import (
	"net/url"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/fromforgesoftware/go-kit/monitoring"

	"github.com/fromforgesoftware/go-kit/persistence/gormdb"
	"github.com/fromforgesoftware/go-kit/persistence/sqldb"
)

// parseQueryExecMode converts the DB_QUERY_EXEC_MODE environment variable to pgx.QueryExecMode
func parseQueryExecMode() pgx.QueryExecMode {
	mode := os.Getenv("DB_QUERY_EXEC_MODE")
	switch mode {
	case "simple_protocol":
		return pgx.QueryExecModeSimpleProtocol
	case "cache_statement":
		return pgx.QueryExecModeCacheStatement
	case "cache_describe":
		return pgx.QueryExecModeCacheDescribe
	case "describe_exec":
		return pgx.QueryExecModeDescribeExec
	case "exec":
		return pgx.QueryExecModeExec
	default:
		// Default to simple protocol if not specified or invalid
		return pgx.QueryExecModeCacheStatement
	}
}

// NewClient creates a new instance of a gorm client connected to the postgres database
// given by the sql.DB conn. Optionally, a set of gormdb.Option can be provided
// to define extra behaviour on client initialization/interaction.
//
// The list of default applied options applied by gormdb.New
// (but can be overiden ):
//
// - WithSingularTable(true): singular naming convention for tables
//
// - WithSQLConnectionOptions(WithDBSchemaFromEnv, WithMaxOpenLimit(100), WithMaxIdleConns(10)):
// which on client initialization links the SEARCH_PATH (DB_SCHEMA envvar is set) to the schema
// and establishes a maximum of 100 connections for the pool, and a max of 10 idle conns.
func NewClient(connURL *url.URL, m monitoring.Monitor, options ...gormdb.Option) (*gormdb.DBClient, error) {
	if connURL == nil {
		return nil, sqldb.NewErrEmptyDBConnection()
	}

	config, err := pgx.ParseConfig(connURL.String())
	if err != nil {
		return nil, err
	}
	config.DefaultQueryExecMode = parseQueryExecMode()
	db := stdlib.OpenDB(*config)

	return gormdb.New(
		postgres.New(postgres.Config{Conn: db}), m,
		options...,
	)
}

// PreloadRecursively returns a function that applies the GORM Preload method recursively
// for the specified association name and its nested associations. This can be used to
// eagerly load related data in a hierarchical or nested structure.
//
// Example usage:
//
//	err := db.Preload("Parent", PreloadRecursively("Parent")).First(&dialogue).Error
//
// In the example above, PreloadRecursively is used to recursively preload the "Parent"
// association along with its nested associations for the "dialogue" model.
//
// Parameters:
//   - name: The name of the association to preload.
//
// Returns:
//
//	A function that takes a *gorm.DB instance and returns the same instance with the
//	Preload method applied recursively for the specified association and its nested
//	associations.
//
// Note: As an alternative, if you know in advance the maximum depth of the query, you can use Joins for query optimization, like this:
//
//	err = repo.QueryApplyWithTableName(queryFilter, "test_resource").
//		WithContext(ctx).
//		Joins("Parent.Parent.Parent.Parent"). // Knowing the depth in advance for query optimization.
//		First(&res).Error
func PreloadRecursively(name string) func(d *gorm.DB) *gorm.DB {
	return func(d *gorm.DB) *gorm.DB {
		return d.Preload(name, PreloadRecursively(name))
	}
}
