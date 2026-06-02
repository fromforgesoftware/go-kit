//go:build integration
// +build integration

package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/fromforgesoftware/go-kit/filter"
	"github.com/fromforgesoftware/go-kit/persistence/gormdb/gormdbtest"
	"github.com/fromforgesoftware/go-kit/search/query"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEntity is a simple model for testing repository logic
type TestEntity struct {
	EID        string    `gorm:"primaryKey;column:id"`
	EName      string    `gorm:"column:name"`
	EAge       int       `gorm:"column:age"`
	ECreatedAt time.Time `gorm:"column:created_at"`
}

func (TestEntity) TableName() string {
	return "test_entities"
}

func TestRepoFilterApplyIntegration(t *testing.T) {
	// Spin up test DB container
	testDB := gormdbtest.GetDB(t, gormdbtest.TestSchema)
	require.NotNil(t, testDB)

	// Create table for test entity manually since we don't have migrations for it
	err := testDB.DB.AutoMigrate(&TestEntity{})
	require.NoError(t, err)

	// Seed data
	now := time.Now().UTC().Truncate(time.Microsecond)
	seedData := []TestEntity{
		{EID: "1", EName: "Alice", EAge: 30, ECreatedAt: now},
		{EID: "2", EName: "Bob", EAge: 20, ECreatedAt: now.Add(-1 * time.Hour)},
		{EID: "3", EName: "Charlie", EAge: 25, ECreatedAt: now.Add(-2 * time.Hour)},
	}
	require.NoError(t, testDB.DB.Create(&seedData).Error)

	// Init Repo
	repo, err := NewRepo(
		testDB.DBClient,
		map[string]string{
			"id":         "id",
			"name":       "name",
			"age":        "age",
			"created_at": "created_at",
		},
	)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("OpEq", func(t *testing.T) {
		q := query.New()
		query.AddFilter(q, filter.OpEq, "name", "Alice")

		var results []TestEntity
		err := repo.QueryApply(ctx, q).Find(&results).Error
		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, "Alice", results[0].EName)
	})

	t.Run("OpGT", func(t *testing.T) {
		q := query.New()
		query.AddFilter(q, filter.OpGT, "age", 22)

		var results []TestEntity
		err := repo.QueryApply(ctx, q).Order("age ASC").Find(&results).Error
		require.NoError(t, err)
		assert.Len(t, results, 2) // Charlie (25), Alice (30)
		assert.Equal(t, "Charlie", results[0].EName)
		assert.Equal(t, "Alice", results[1].EName)
	})

	t.Run("OpBetween", func(t *testing.T) {
		q := query.New()
		query.AddFilter(q, filter.OpBetween, "age", []any{20, 28})

		var results []TestEntity
		err := repo.QueryApply(ctx, q).Order("age ASC").Find(&results).Error
		require.NoError(t, err)
		assert.Len(t, results, 2) // Bob (20), Charlie (25)
		assert.Equal(t, "Bob", results[0].EName)
		assert.Equal(t, "Charlie", results[1].EName)
	})

	t.Run("OpIn", func(t *testing.T) {
		q := query.New()
		query.AddFilter(q, filter.OpIn, "name", []string{"Alice", "Bob"})

		var results []TestEntity
		err := repo.QueryApply(ctx, q).Order("name ASC").Find(&results).Error
		require.NoError(t, err)
		assert.Len(t, results, 2)
		assert.Equal(t, "Alice", results[0].EName)
		assert.Equal(t, "Bob", results[1].EName)
	})

	t.Run("Sorting", func(t *testing.T) {
		q := query.New()
		q.Sorting().Set("age", query.SortDesc)

		var results []TestEntity
		err := repo.QueryApply(ctx, q).Find(&results).Error
		require.NoError(t, err)
		assert.Equal(t, "Alice", results[0].EName)   // 30
		assert.Equal(t, "Charlie", results[1].EName) // 25
		assert.Equal(t, "Bob", results[2].EName)     // 20
	})

	t.Run("Pagination", func(t *testing.T) {
		q := query.New(query.Pagination(1, 0))
		q.Sorting().Set("age", query.SortAsc)

		var results []TestEntity
		err := repo.QueryApply(ctx, q).Find(&results).Error
		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, "Bob", results[0].EName)

		q2 := query.New(query.Pagination(1, 1))
		q2.Sorting().Set("age", query.SortAsc)
		err = repo.QueryApply(ctx, q2).Find(&results).Error
		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, "Charlie", results[0].EName)
	})
}

func TestRepoPatchApplyIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	testDB := gormdbtest.GetDB(t, gormdbtest.TestSchema)
	require.NotNil(t, testDB)
	require.NoError(t, testDB.DB.AutoMigrate(&TestEntity{}))

	// Seed one record
	seed := TestEntity{EID: "patch-1", EName: "Original", EAge: 50, ECreatedAt: time.Now()}
	require.NoError(t, testDB.DB.Create(&seed).Error)

	// Mapper has "name" -> "name", "age" -> "age"
	// But let's verify if we had a mapping like "external_age" -> "age"
	// For now, we test the existing mapper: "name" -> "name"
	repo, err := NewRepo(
		testDB.DBClient,
		map[string]string{
			"id":           "id",
			"name":         "name",
			"external_age": "age",
		},
	)
	require.NoError(t, err)

	ctx := context.Background()

	// Case 1: Patch using mapped key "external_age" -> should update "age"
	toPatch := map[string]any{
		"name":         "Patched Name",
		"external_age": 55,
	}

	err = repo.PatchApply(ctx, nil, &TestEntity{EID: "patch-1"}, toPatch).Error
	require.NoError(t, err)

	// Verify update
	var loaded TestEntity
	err = testDB.DB.First(&loaded, "id = ?", "patch-1").Error
	require.NoError(t, err)
	assert.Equal(t, "Patched Name", loaded.EName)
	assert.Equal(t, 55, loaded.EAge)
}

func TestRepoCountApplyIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	testDB := gormdbtest.GetDB(t, gormdbtest.TestSchema)
	require.NotNil(t, testDB)
	require.NoError(t, testDB.DB.AutoMigrate(&TestEntity{}))

	// Seed records
	seedData := []TestEntity{
		{EID: "c1", EName: "A", EAge: 10},
		{EID: "c2", EName: "A", EAge: 20},
		{EID: "c3", EName: "B", EAge: 30},
	}
	require.NoError(t, testDB.DB.Create(&seedData).Error)

	repo, err := NewRepo(
		testDB.DBClient,
		map[string]string{"name": "name"},
	)
	require.NoError(t, err)

	ctx := context.Background()

	q := query.New()
	query.AddFilter(q, filter.OpEq, "name", "A")

	var count int64
	err = repo.CountApply(ctx, &TestEntity{}, q).Count(&count).Error
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}
