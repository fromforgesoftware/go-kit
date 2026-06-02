// Package gormpg provides the PostgreSQL-specific implementation for the gormdb client.
//
// It contains the factory logic to initialize a gormdb.DBClient using the PostgreSQL driver
// and proper DSN configuration. It also provides an Fx module for easy dependency injection
// of the database client and transactioner.
package gormpg
