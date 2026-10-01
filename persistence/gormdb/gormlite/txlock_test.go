package gormlite_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/fromforgesoftware/go-kit/monitoring/monitoringtest"
	"github.com/fromforgesoftware/go-kit/persistence/gormdb/gormlite"
)

// A deferred SQLite transaction that reads before it writes cannot upgrade once another
// connection has written, and fails with SQLITE_BUSY at once instead of waiting. The DSN opens
// every transaction immediate, so the second writer is the one that waits.
func TestWriteTransactionsTakeTheLockUpFront(t *testing.T) {
	t.Parallel()
	client, err := gormlite.NewClient(filepath.Join(t.TempDir(), "txlock.db"), monitoringtest.NewMonitor(t))
	require.NoError(t, err)
	require.NoError(t, client.Exec("CREATE TABLE counters (id INTEGER PRIMARY KEY, n INTEGER NOT NULL)").Error)
	require.NoError(t, client.Exec("INSERT INTO counters (id, n) VALUES (1, 0)").Error)

	read := make(chan struct{})
	proceed := make(chan struct{})
	first := make(chan error, 1)
	go func() {
		first <- client.Transaction(func(tx *gorm.DB) error {
			var n int
			if err := tx.Raw("SELECT n FROM counters WHERE id = 1").Scan(&n).Error; err != nil {
				return err
			}
			close(read)
			<-proceed
			return tx.Exec("UPDATE counters SET n = ? WHERE id = 1", n+1).Error
		})
	}()
	<-read

	second := make(chan error, 1)
	go func() { second <- client.Exec("UPDATE counters SET n = n + 10 WHERE id = 1").Error }()
	select {
	case err := <-second:
		t.Fatalf("the second writer slipped in under an open transaction: %v", err)
	case <-time.After(200 * time.Millisecond):
	}

	close(proceed)
	require.NoError(t, <-first)
	require.NoError(t, <-second)
	var n int
	require.NoError(t, client.Raw("SELECT n FROM counters WHERE id = 1").Scan(&n).Error)
	assert.Equal(t, 11, n, "both writes landed, in order")
}
