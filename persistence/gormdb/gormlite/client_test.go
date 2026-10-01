package gormlite_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/monitoring/logger"
	"github.com/fromforgesoftware/go-kit/monitoring/monitoringtest"
	"github.com/fromforgesoftware/go-kit/persistence/gormdb"
	"github.com/fromforgesoftware/go-kit/persistence/gormdb/gormlite"
)

type account struct {
	ID        string    `gorm:"primaryKey;column:id"`
	Email     string    `gorm:"column:email;uniqueIndex"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (account) TableName() string { return "account" }

func TestNewClientRefusesAnEmptyPath(t *testing.T) {
	t.Parallel()

	_, err := gormlite.NewClient("", monitoringtest.NewMonitor(t))

	require.Error(t, err)
}

func TestNewClientAppliesThePragmas(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)

	pragmas := map[string]string{}
	for _, name := range []string{"journal_mode", "synchronous", "foreign_keys"} {
		var value string
		require.NoError(t, db.Raw("PRAGMA "+name).Scan(&value).Error)
		pragmas[name] = value
	}

	assert.Equal(t, map[string]string{
		"journal_mode": "wal",
		"synchronous":  "2",
		"foreign_keys": "1",
	}, pragmas)
}

func TestTimesRoundTripInUTC(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	createdAt := time.Date(2026, 10, 1, 9, 30, 15, 123456000, time.UTC)
	require.NoError(t, db.Create(&account{ID: "a1", Email: "a@example.com", CreatedAt: createdAt}).Error)

	var got account
	require.NoError(t, db.First(&got, "id = ?", "a1").Error)

	assert.True(t, got.CreatedAt.Equal(createdAt), "stored %s, read %s", createdAt, got.CreatedAt)
}

func TestIsDuplicateKey(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	require.NoError(t, db.Create(&account{ID: "a1", Email: "a@example.com"}).Error)

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "unique index", err: db.Create(&account{ID: "a2", Email: "a@example.com"}).Error, want: true},
		{name: "primary key", err: db.Create(&account{ID: "a1", Email: "b@example.com"}).Error, want: true},
		{name: "no violation", err: db.Create(&account{ID: "a3", Email: "c@example.com"}).Error, want: false},
		{name: "unrelated error", err: errors.New("boom"), want: false},
		{name: "nil", err: nil, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, gormlite.IsDuplicateKey(tt.err))
		})
	}
}

func TestTransactionerRollsBackOnError(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	tx := gormdb.NewTransactioner(db, logger.New())
	ctx := context.Background()

	err := tx.Exec(ctx, func(ctx context.Context) error {
		if err := db.WithContext(ctx).Create(&account{ID: "a1", Email: "a@example.com"}).Error; err != nil {
			return err
		}
		return errors.New("abort")
	})
	require.EqualError(t, err, "abort")

	var count int64
	require.NoError(t, db.Model(&account{}).Count(&count).Error)
	assert.Equal(t, int64(0), count)
}

func TestTransactionerCommitsOnSuccess(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	tx := gormdb.NewTransactioner(db, logger.New())
	ctx := context.Background()

	err := tx.Exec(ctx, func(ctx context.Context) error {
		return db.WithContext(ctx).Create(&account{ID: "a1", Email: "a@example.com"}).Error
	})
	require.NoError(t, err)

	var count int64
	require.NoError(t, db.Model(&account{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestConcurrentWritersSerialiseInsteadOfFailing(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	const writers = 8
	errs := make(chan error, writers)
	for i := 0; i < writers; i++ {
		go func(i int) {
			errs <- db.Create(&account{ID: string(rune('a' + i)), Email: string(rune('a'+i)) + "@example.com"}).Error
		}(i)
	}
	for i := 0; i < writers; i++ {
		require.NoError(t, <-errs)
	}

	var count int64
	require.NoError(t, db.Model(&account{}).Count(&count).Error)
	assert.Equal(t, int64(writers), count)
}

func BenchmarkInsertSynchronousFull(b *testing.B) {
	db := openBenchDB(b)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		if err := db.Create(&account{ID: benchID(i), Email: benchID(i) + "@example.com"}).Error; err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPointReadByPrimaryKey(b *testing.B) {
	db := openBenchDB(b)
	for i := 0; i < 1000; i++ {
		if err := db.Create(&account{ID: benchID(i), Email: benchID(i) + "@example.com"}).Error; err != nil {
			b.Fatal(err)
		}
	}
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		var got account
		if err := db.First(&got, "id = ?", benchID(i%1000)).Error; err != nil {
			b.Fatal(err)
		}
	}
}

func benchID(i int) string {
	const digits = "0123456789"
	out := make([]byte, 0, 8)
	for i > 0 || len(out) == 0 {
		out = append([]byte{digits[i%10]}, out...)
		i /= 10
	}
	return "id" + string(out)
}

func openTestDB(t *testing.T) *gormdb.DBClient {
	t.Helper()
	db, err := gormlite.NewClient(filepath.Join(t.TempDir(), "test.db"), monitoringtest.NewMonitor(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.AutoMigrate(&account{}))
	return db
}

func openBenchDB(b *testing.B) *gormdb.DBClient {
	b.Helper()
	db, err := gormlite.NewClient(filepath.Join(b.TempDir(), "bench.db"), monitoringtest.NewMonitor(b))
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = db.Close() })
	if err := db.AutoMigrate(&account{}); err != nil {
		b.Fatal(err)
	}
	return db
}
