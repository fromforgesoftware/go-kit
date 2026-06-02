// Package sqldb provides the core foundation for SQL database interactions within the mmo-game architecture.
//
// It includes functionality for:
//   - Connection management (pooling, health checks) via Configurable DSN parameters
//   - Transaction management (Begin, Commit, Rollback, context propagation)
//   - Prepared statement registry for performance optimization
//   - High-performance connection pool configuration presets
//
// The package is designed to work with the "database/sql" standard library and provides
// a "DBClient" wrapper to extend functionality with project-specific patterns.
package sqldb
