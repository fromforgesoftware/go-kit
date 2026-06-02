package sqldb

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
)

// ConnectionOption defines the contract for options applied to a sql.DB.
type ConnectionOption func(db *sql.DB) error

// WithMaxOpenLimit sets the maximum number of open connections to the database.
// If limit is <= 0, it is ignored.
func WithMaxOpenLimit(limit int) ConnectionOption {
	return func(db *sql.DB) error {
		if limit > 0 {
			db.SetMaxOpenConns(limit)
		}
		return nil
	}
}

// WithMaxIdleConns sets the maximum number of idle connections in the pool.
// If limit is <= 0, it is ignored.
func WithMaxIdleConns(limit int) ConnectionOption {
	return func(db *sql.DB) error {
		if limit > 0 {
			db.SetMaxIdleConns(limit)
		}
		return nil
	}
}

// WithDBSchema sets the database schema search path for PostgreSQL.
// The schema is added to the search path along with 'public'.
// If schema is empty or whitespace, this option does nothing.
func WithDBSchema(schema string) ConnectionOption {
	return func(db *sql.DB) error {
		schema = strings.TrimSpace(schema)
		if schema == "" {
			return nil
		}
		_, err := db.Exec(fmt.Sprintf("SET SEARCH_PATH to %q,public", schema))
		return err
	}
}

// WithDBSchemaFromEnv sets the database schema by reading the DB_SCHEMA environment variable.
func WithDBSchemaFromEnv() ConnectionOption {
	return WithDBSchema(os.Getenv("DB_SCHEMA"))
}

// Connect establishes a connection to a sql.DB using the given DSN URL.
func Connect(connURL *url.URL) (*sql.DB, error) {
	if connURL == nil || connURL.String() == "" {
		return nil, newErrConnEmptyDSN()
	}
	driverType := DriverType(connURL.Scheme)
	if !driverType.valid() {
		return nil, newErrConnInvalidDriver(driverType)
	}

	db, err := sql.Open(string(driverType), connURL.String())
	if err != nil {
		fmt.Println("[DEBUG] sql.Open error:", err)
		fmt.Println("[DEBUG] DSN:", connURL.String())
		return nil, newErrConn(err)
	}

	// Test the connection
	err = db.Ping()
	if err != nil {
		fmt.Println("[DEBUG] db.Ping error:", err)
		fmt.Println("[DEBUG] DSN:", connURL.String())
		return nil, newErrConn(err)
	}

	return db, nil
}

// MustConnectWithDSN ensures a connection to a *sql.DB with the given dsn url,
// else it panics.
func MustConnectWithDSN(dsn *url.URL) *sql.DB {
	db, err := Connect(dsn)
	if err != nil {
		panic(err)
	}

	return db
}
