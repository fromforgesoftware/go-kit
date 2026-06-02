// Package fixtures provides simple SQL fixture loading from embedded filesystems.
//
// All fixture files MUST follow the timestamp naming convention:
// YYYYMMDDHHmmss_name.sql
//
// Files are executed in chronological order within a single transaction.
//
// # Basic Usage
//
//	//go:embed fixtures
//	var fixturesFS embed.FS
//
//	func main() {
//	    if err := fixtures.Load(context.Background(), fixturesFS); err != nil {
//	        log.Fatal(err)
//	    }
//	}
//
// # Naming Convention
//
// Fixture files MUST be named with a timestamp prefix (YYYYMMDDHHmmss):
//   - 20250123120000_create_users.sql
//   - 20250123120100_create_posts.sql
//   - 20250123120200_seed_data.sql
//
// This follows the same pattern as database migrations, ensuring
// chronological ordering and avoiding numbering conflicts.
//
// # Transaction Guarantee
//
// All fixtures are loaded within a single transaction using the
// Transactioner interface. If any file fails, the entire operation
// is rolled back.
package fixtures
