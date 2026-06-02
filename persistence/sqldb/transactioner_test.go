//go:build integration
// +build integration

package sqldb_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fromforgesoftware/go-kit/persistence/sqldb"
	"github.com/fromforgesoftware/go-kit/persistence/sqldb/sqldbtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransactionerIntegration(t *testing.T) {
	testDB := sqldbtest.GetDB(t, sqldbtest.TestSchema)
	db := testDB.DB()

	// Ensure schema exists
	_, err := db.Exec("CREATE SCHEMA IF NOT EXISTS " + testDB.Schema)
	require.NoError(t, err)

	// Use the schema from testDB to ensure table is created in the right place
	tableName := testDB.Schema + ".tx_test"

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS " + tableName + " (id INT PRIMARY KEY, val INT)")
	require.NoError(t, err)
	// Truncate to be clean
	_, err = db.Exec("TRUNCATE TABLE " + tableName)
	require.NoError(t, err)

	trx := sqldb.NewTransactioner(db)
	ctx := context.Background()

	t.Run("Success commit", func(t *testing.T) {
		err := trx.Exec(ctx, func(ctx context.Context) error {
			// Insert value
			_, err := sqldb.GetTx(ctx, db).ExecContext(ctx, "INSERT INTO "+tableName+" (id, val) VALUES (1, 100)")
			return err
		})
		require.NoError(t, err)

		// Verify committed
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM " + tableName + " WHERE id = 1").Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("Failure rollback", func(t *testing.T) {
		err := trx.Exec(ctx, func(ctx context.Context) error {
			// Insert value
			_, err := sqldb.GetTx(ctx, db).ExecContext(ctx, "INSERT INTO "+tableName+" (id, val) VALUES (2, 200)")
			require.NoError(t, err)
			return errors.New("abort")
		})
		require.Error(t, err)

		// Verify NOT committed
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM " + tableName + " WHERE id = 2").Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("Nested transaction", func(t *testing.T) {
		err := trx.Exec(ctx, func(ctx context.Context) error {
			_, err := sqldb.GetTx(ctx, db).ExecContext(ctx, "INSERT INTO "+tableName+" (id, val) VALUES (3, 300)")
			require.NoError(t, err)

			// Nested call
			return trx.Exec(ctx, func(ctx context.Context) error {
				_, err := sqldb.GetTx(ctx, db).ExecContext(ctx, "INSERT INTO "+tableName+" (id, val) VALUES (4, 400)")
				return err
			})
		})
		require.NoError(t, err)

		// Verify both committed
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM " + tableName + " WHERE id IN (3, 4)").Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})
}
