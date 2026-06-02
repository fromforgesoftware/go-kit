// Package gormdb provides the database-agnostic core for GORM integration.
//
// It defines the generic DBClient wrapper, configuration options, transaction management,
// and monitoring hooks. This package is designed to be used in conjunction with a
// driver-specific implementation (e.g., gormpg for PostgreSQL, gormmysql for MySQL),
// which provides the specific dialector and connection logic.
//
// This separation of concerns allows for easy extensibility to support multiple
// database drivers while reusing the core GORM interaction logic.
package gormdb
