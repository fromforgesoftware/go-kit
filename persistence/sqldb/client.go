package sqldb

import (
	"context"
	"database/sql"
	"io"
)

// DatabaseProvider defines the contract to retrieve the sql.DB handle.
type DatabaseProvider interface {
	Database() (*sql.DB, error)
}

// Pinger defines the contract to ping a database (helpful for healthchecking purposes).
type Pinger interface {
	Ping() error
}

// ContextAwarePinger defines the contract to ping a database with the
// given context (helpful for healthchecking purposes).
type ContextAwarePinger interface {
	PingContext(context.Context) error
}

// Client defines the contract of a client to interact with a sql database.
type Client interface {
	DatabaseProvider
	io.Closer
	Pinger
	ContextAwarePinger
}
