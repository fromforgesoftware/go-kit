//go:build integration
// +build integration

package fixtures_test

import (
	"context"
	"embed"
	"io/fs"
	"testing"

	"github.com/fromforgesoftware/go-kit/fixtures"
	"github.com/fromforgesoftware/go-kit/monitoring/logger"
	"github.com/fromforgesoftware/go-kit/persistence/sqldb"
	"github.com/fromforgesoftware/go-kit/persistence/sqldb/sqldbtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed testdata
var testFixturesFS embed.FS

// TestLoadIntegration tests loading fixtures with a real database and migrations.
func TestLoadIntegration(t *testing.T) {
	ctx := context.Background()

	// Get test database with test schema set as search_path
	testDB := sqldbtest.GetDB(t, sqldbtest.TestSchema)

	// Wrap embed.FS to read from testdata subdirectory
	subFS, err := fs.Sub(testFixturesFS, "testdata")
	require.NoError(t, err, "Failed to get testdata subdirectory")

	// Create loader with test database
	loader, err := fixtures.New(
		sqldb.NewDBClient(testDB.DB()),
		fixtures.WithLogger(logger.New()),
	)
	require.NoError(t, err)

	// Load fixtures
	err = loader.Load(ctx, subFS)
	require.NoError(t, err)

	// Verify fixtures were loaded correctly (3 from 0001 + 2 from 0002 = 5 total)
	var count int
	err = testDB.QueryRow(ctx, "SELECT COUNT(*) FROM fixtures_test").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 5, count, "Expected 5 rows total from both fixture files")

	// Verify specific data from first fixture
	var name, value string
	err = testDB.QueryRow(ctx, "SELECT name, value FROM fixtures_test WHERE name = $1", "fixture_1").Scan(&name, &value)
	require.NoError(t, err)
	assert.Equal(t, "fixture_1", name)
	assert.Equal(t, "Test value 1", value)

	// Verify data from second fixture
	err = testDB.QueryRow(ctx, "SELECT name, value FROM fixtures_test WHERE name = $1", "fixture_4").Scan(&name, &value)
	require.NoError(t, err)
	assert.Equal(t, "fixture_4", name)
	assert.Equal(t, "Test value 4", value)

	// Verify ordering - all 5 rows should be present
	var names []string
	rows, err := testDB.Query(ctx, "SELECT name FROM fixtures_test ORDER BY name")
	require.NoError(t, err)
	defer rows.Close()

	for rows.Next() {
		var n string
		require.NoError(t, rows.Scan(&n))
		names = append(names, n)
	}
	assert.Equal(t, []string{"fixture_1", "fixture_2", "fixture_3", "fixture_4", "fixture_5"}, names)
}
