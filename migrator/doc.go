// Package migrator provides database migration utilities.
//
// This package wraps golang-migrate/migrate and provides a simple API
// for running database migrations with optional pre/post migration scripts.
//
// # Basic Usage
//
//	//go:embed migrations
//	var migrationsFS embed.FS
//
//	func main() {
//	    if err := migrator.Up(context.Background(), migrationsFS); err != nil {
//	        log.Fatal(err)
//	    }
//	}
//
// # Directory Structure
//
// The migrations filesystem should have this structure:
//   - migrations/*.sql                         (actual migrations, e.g., 000001_init.up.sql)
//   - migrations/common-pre-migration/*.sql    (optional pre-migration scripts)
//   - migrations/common-post-migration/*.sql   (optional post-migration scripts)
//
// # Migration Files
//
// Migration files follow golang-migrate naming convention:
//   - {version}_{description}.up.sql    (up migrations)
//   - {version}_{description}.down.sql  (down migrations - not used in up-only mode)
//
// Example:
//   - 000001_create_users.up.sql
//   - 000002_add_email_index.up.sql
//
// # Advanced Usage
//
//	migrator, _ := migrator.New(
//	    migrator.WithServiceName("myservice"),
//	    migrator.WithLogger(logger.New()),
//	    migrator.WithPreScripts(true),
//	    migrator.WithPostScripts(true),
//	)
//	defer migrator.Close()
//
//	migrator.Run(ctx, migrationsFS)
//
// # Features
//
//   - Uses golang-migrate/migrate for migration management
//   - Service-specific migration tracking table ({service}_schema_migrations)
//   - Optional pre/post migration script execution
//   - Automatic sorting and execution order
//   - Transaction support per migration file
//   - Integration with sqldb package
package migrator
