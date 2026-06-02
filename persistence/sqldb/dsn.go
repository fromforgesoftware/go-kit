package sqldb

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
)

type connField string

func (f connField) String() string {
	return string(f)
}

func (f connField) valid() bool {
	return f == connFieldHost || f == connFieldPort ||
		f == connFieldUser || f == connFieldPwd ||
		f == connFieldDBName || f == connFieldSSLMode ||
		f == connFieldSearchPath
}

const (
	connFieldHost       connField = "host"
	connFieldPort       connField = "port"
	connFieldUser       connField = "user"
	connFieldPwd        connField = "password"
	connFieldDBName     connField = "dbname"
	connFieldSSLMode    connField = "sslmode"
	connFieldSearchPath connField = "search_path"
)

//nolint:gochecknoglobals // we want this global variable to have all the connection fields in place
var orderedConnFields = []connField{
	connFieldHost, connFieldPort, connFieldDBName,
	connFieldUser, connFieldPwd, connFieldSSLMode,
}

type dsn map[connField]string

func (d dsn) add(f connField, val string) error {
	if !f.valid() {
		return nil
	}
	d[f] = strings.TrimSpace(val)

	return nil
}

func (d dsn) buildQueryParams() url.Values {
	vals := url.Values{}
	vals.Add(connFieldSSLMode.String(), d[connFieldSSLMode])

	// lib/pq uses 'options' to pass session-level parameters like search_path
	if searchPath := d[connFieldSearchPath]; searchPath != "" {
		vals.Add("options", fmt.Sprintf("-c %s=%s", connFieldSearchPath.String(), searchPath))
	}

	return vals
}

func (d dsn) validateConnFields() error {
	for _, f := range orderedConnFields {
		if d[f] == "" {
			return newErrEmptyDSNField(f)
		}
	}
	return nil
}

func (d dsn) genConnectionURL(driverType DriverType) (*url.URL, error) {
	if err := d.validateConnFields(); err != nil {
		return nil, err
	}

	return &url.URL{
		Scheme:   string(driverType),
		User:     url.UserPassword(d[connFieldUser], d[connFieldPwd]),
		Path:     d[connFieldDBName],
		Host:     net.JoinHostPort(d[connFieldHost], d[connFieldPort]),
		RawQuery: d.buildQueryParams().Encode(),
	}, nil
}

// ConnectionDSNOption defines the contract for the options being applied to the DSN.
type ConnectionDSNOption func(dsn) error

func withConnStringVal(field connField, val string) ConnectionDSNOption {
	return func(connDSN dsn) error {
		return connDSN.add(field, val)
	}
}

// WithConnHost sets the database host in the DSN.
func WithConnHost(host string) ConnectionDSNOption {
	return withConnStringVal(connFieldHost, host)
}

// WithConnPort sets the database port in the DSN.
func WithConnPort(port string) ConnectionDSNOption {
	return withConnStringVal(connFieldPort, port)
}

// WithConnUser sets the database user in the DSN.
func WithConnUser(user string) ConnectionDSNOption {
	return withConnStringVal(connFieldUser, user)
}

// WithConnPwd sets the database password in the DSN.
func WithConnPwd(pwd string) ConnectionDSNOption {
	return withConnStringVal(connFieldPwd, pwd)
}

// WithConnDBName sets the database name in the DSN.
func WithConnDBName(dbName string) ConnectionDSNOption {
	return withConnStringVal(connFieldDBName, dbName)
}

// WithConnSSLMode sets the SSL mode in the DSN (e.g., "disable", "require", "verify-full").
func WithConnSSLMode(sslMode string) ConnectionDSNOption {
	return withConnStringVal(connFieldSSLMode, sslMode)
}

// WithConnSearchPath sets the PostgreSQL search_path in the DSN.
// This is useful for multi-schema databases where you want to set a default schema.
func WithConnSearchPath(searchPath string) ConnectionDSNOption {
	return withConnStringVal(connFieldSearchPath, searchPath)
}

// WithDSNConnFromEnv loads all DSN parameters from environment variables.
// This is the default option applied when calling NewDSN.
//
// Environment variables read:
//   - DB_HOST: Database host
//   - DB_PORT: Database port
//   - DB_NAME: Database name
//   - DB_USER: Database user
//   - DB_PASSWORD: Database password
//   - DB_SSL: SSL mode
func WithDSNConnFromEnv() ConnectionDSNOption {
	return func(dsn dsn) error {
		options := []ConnectionDSNOption{
			WithConnHost(os.Getenv("DB_HOST")),
			WithConnPort(os.Getenv("DB_PORT")),
			WithConnDBName(os.Getenv("DB_NAME")),
			WithConnUser(os.Getenv("DB_USER")),
			WithConnPwd(os.Getenv("DB_PASSWORD")),
			WithConnSSLMode(os.Getenv("DB_SSL")),
			WithConnSearchPath(ensurePublicSchema(os.Getenv("DB_SCHEMA"))),
		}

		for _, opt := range options {
			err := opt(dsn)
			if err != nil {
				return err
			}
		}

		return nil
	}
}

func defaultDSNOptions() []ConnectionDSNOption {
	return []ConnectionDSNOption{
		WithDSNConnFromEnv(),
	}
}

// NewDSN generates a DSN (Data Source Name) URL for database connections.
//
// By default, it reads connection parameters from environment variables:
//   - DB_HOST, DB_PORT, DB_NAME, DB_USER, DB_PASSWORD, DB_SSL
//
// These defaults can be overridden by passing specific ConnectionDSNOption functions.
//
// Example:
//
//	dsn, err := NewDSN(
//	    DriverTypePostgres,
//	    WithConnHost("localhost"),
//	    WithConnPort("5432"),
//	)
func NewDSN(driverType DriverType, options ...ConnectionDSNOption) (*url.URL, error) {
	if !driverType.valid() {
		return nil, newErrConnInvalidDriver(driverType)
	}

	connDSN := dsn(map[connField]string{
		connFieldHost: "", connFieldPort: "",
		connFieldDBName: "", connFieldUser: "",
		connFieldPwd: "", connFieldSSLMode: "",
	})
	for _, opt := range append(defaultDSNOptions(), options...) {
		err := opt(connDSN)
		if err != nil {
			return nil, err
		}
	}

	return connDSN.genConnectionURL(driverType)
}

// MustGenerateDSN generates a DSN URL or panics if an error occurs.
// This is useful for initialization code where errors should be fatal.
func MustGenerateDSN(driverType DriverType, options ...ConnectionDSNOption) *url.URL {
	dsn, err := NewDSN(driverType, options...)
	if err != nil {
		panic(err)
	}

	return dsn
}
func ensurePublicSchema(schema string) string {
	if schema != "" && !strings.Contains(schema, "public") {
		return schema + ",public"
	}
	return schema
}
