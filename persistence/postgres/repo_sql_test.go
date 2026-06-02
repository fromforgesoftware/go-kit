package postgres_test

import (
	"context"
	"database/sql/driver"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/fromforgesoftware/go-kit/filter"
	"github.com/fromforgesoftware/go-kit/monitoring/monitoringtest"
	"github.com/fromforgesoftware/go-kit/persistence/gormdb"
	pgrepo "github.com/fromforgesoftware/go-kit/persistence/postgres"
	"github.com/fromforgesoftware/go-kit/search/query"
)

type SQLTestEntity struct {
	ID        string    `gorm:"primaryKey;column:id"`
	Name      string    `gorm:"column:name"`
	Age       int       `gorm:"column:age"`
	Status    string    `gorm:"column:status"`
	Amount    float64   `gorm:"column:amount"`
	Category  string    `gorm:"column:category"`
	Region    string    `gorm:"column:region"`
	DeletedAt *string   `gorm:"column:deleted_at"`
	Tags      []string  `gorm:"column:tags;type:text[]"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (SQLTestEntity) TableName() string { return "events" }

func fieldMap() map[string]string {
	return map[string]string{
		"id":        "id",
		"name":      "name",
		"age":       "age",
		"status":    "status",
		"amount":    "amount",
		"category":  "category",
		"region":    "region",
		"deletedAt": "deleted_at",
		"tags":      "tags",
		"createdAt": "created_at",
		"createdBy": "created_by",
		"assignee":  "assignee",
	}
}

func newMockRepo(t *testing.T) (*pgrepo.Repo, *gormdb.DBClient, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(
		sqlmock.MonitorPingsOption(true),
		sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp),
	)
	require.NoError(t, err)
	mock.ExpectPing()

	t.Setenv("ENV", "local")
	t.Setenv("LOG_LEVEL", "warn")
	t.Setenv("DB_LOG_LEVEL", "warn")

	cli, err := gormdb.New(
		postgres.New(postgres.Config{Conn: db}),
		monitoringtest.NewMonitor(t),
	)
	require.NoError(t, err)

	cli.DB = cli.DB.Session(&gorm.Session{DryRun: true, SkipDefaultTransaction: true, NewDB: true})

	repo, err := pgrepo.NewRepo(cli, fieldMap())
	require.NoError(t, err)
	return repo, cli, mock
}

func captureSQL(t *testing.T, _ *gormdb.DBClient, prepare func(*gorm.DB) *gorm.DB) string {
	t.Helper()
	var out []SQLTestEntity
	tx := prepare(nil).Find(&out)
	return tx.Dialector.Explain(tx.Statement.SQL.String(), tx.Statement.Vars...)
}

func normalize(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func TestSQL_FilterEq(t *testing.T) {
	repo, db, _ := newMockRepo(t)
	q := query.New(query.FilterBy(filter.OpEq, "status", "active"))
	sql := captureSQL(t, db, func(tx *gorm.DB) *gorm.DB {
		return repo.QueryApply(context.Background(), q).Model(&SQLTestEntity{})
	})
	assert.Contains(t, normalize(sql), "WHERE status = 'active'")
}

func TestSQL_FilterNeq(t *testing.T) {
	repo, db, _ := newMockRepo(t)
	q := query.New(query.FilterBy(filter.OpNEq, "status", "deleted"))
	sql := captureSQL(t, db, func(tx *gorm.DB) *gorm.DB {
		return repo.QueryApply(context.Background(), q).Model(&SQLTestEntity{})
	})
	assert.Contains(t, normalize(sql), "WHERE status <> 'deleted'")
}

func TestSQL_FilterComparisons(t *testing.T) {
	cases := []struct {
		op   filter.Operator
		want string
	}{
		{filter.OpGT, ">"},
		{filter.OpGTEq, ">="},
		{filter.OpLT, "<"},
		{filter.OpLTEq, "<="},
	}
	for _, c := range cases {
		repo, db, _ := newMockRepo(t)
		q := query.New(query.FilterBy(c.op, "age", 18))
		sql := captureSQL(t, db, func(tx *gorm.DB) *gorm.DB {
			return repo.QueryApply(context.Background(), q).Model(&SQLTestEntity{})
		})
		assert.Contains(t, normalize(sql), "age "+c.want+" 18", "op=%s", c.op)
	}
}

func TestSQL_FilterIn(t *testing.T) {
	repo, db, _ := newMockRepo(t)
	q := query.New(query.FilterBy(filter.OpIn, "status", []string{"active", "pending", "archived"}))
	sql := captureSQL(t, db, func(tx *gorm.DB) *gorm.DB {
		return repo.QueryApply(context.Background(), q).Model(&SQLTestEntity{})
	})
	assert.Contains(t, normalize(sql), "status IN ('active','pending','archived')")
}

func TestSQL_FilterNotIn(t *testing.T) {
	repo, db, _ := newMockRepo(t)
	q := query.New(query.FilterBy(filter.OpNotIn, "status", []string{"deleted", "spam"}))
	sql := captureSQL(t, db, func(tx *gorm.DB) *gorm.DB {
		return repo.QueryApply(context.Background(), q).Model(&SQLTestEntity{})
	})
	assert.Contains(t, normalize(sql), "status NOT IN ('deleted','spam')")
}

func TestSQL_FilterLikeWrapsWithPercent(t *testing.T) {
	repo, db, _ := newMockRepo(t)
	q := query.New(query.FilterBy(filter.OpLike, "name", "alice"))
	sql := captureSQL(t, db, func(tx *gorm.DB) *gorm.DB {
		return repo.QueryApply(context.Background(), q).Model(&SQLTestEntity{})
	})
	assert.Contains(t, normalize(sql), "name LIKE '%alice%'")
}

func TestSQL_FilterNotLikeWrapsWithPercent(t *testing.T) {
	repo, db, _ := newMockRepo(t)
	q := query.New(query.FilterBy(filter.OpNotLike, "name", "evil"))
	sql := captureSQL(t, db, func(tx *gorm.DB) *gorm.DB {
		return repo.QueryApply(context.Background(), q).Model(&SQLTestEntity{})
	})
	assert.Contains(t, normalize(sql), "name NOT LIKE '%evil%'")
}

func TestSQL_FilterBetween(t *testing.T) {
	repo, db, _ := newMockRepo(t)
	q := query.New(query.FilterBy(filter.OpBetween, "age", []int{18, 65}))
	sql := captureSQL(t, db, func(tx *gorm.DB) *gorm.DB {
		return repo.QueryApply(context.Background(), q).Model(&SQLTestEntity{})
	})
	assert.Contains(t, normalize(sql), "age BETWEEN 18 AND 65")
}

func TestSQL_FilterIsNull(t *testing.T) {
	repo, db, _ := newMockRepo(t)
	q := query.New(query.FilterBy(filter.OpIsNull, "deletedAt", nil))
	sql := captureSQL(t, db, func(tx *gorm.DB) *gorm.DB {
		return repo.QueryApply(context.Background(), q).Model(&SQLTestEntity{})
	})
	assert.Contains(t, normalize(sql), "deleted_at IS NULL")
}

func TestSQL_FilterNotNull(t *testing.T) {
	repo, db, _ := newMockRepo(t)
	q := query.New(query.FilterBy(filter.OpNotNull, "deletedAt", nil))
	sql := captureSQL(t, db, func(tx *gorm.DB) *gorm.DB {
		return repo.QueryApply(context.Background(), q).Model(&SQLTestEntity{})
	})
	assert.Contains(t, normalize(sql), "deleted_at IS NOT NULL")
}

func TestSQL_FilterContainsArray(t *testing.T) {
	repo, db, _ := newMockRepo(t)
	q := query.New(query.FilterBy(filter.OpContains, "tags", []string{"go", "sql"}))
	sql := captureSQL(t, db, func(tx *gorm.DB) *gorm.DB {
		return repo.QueryApply(context.Background(), q).Model(&SQLTestEntity{})
	})
	assert.Contains(t, normalize(sql), "tags @>")
}

func TestSQL_FilterContainsLike(t *testing.T) {
	repo, db, _ := newMockRepo(t)
	q := query.New(query.FilterBy(filter.OpContainsLike, "tags", []string{"go", "rust"}))
	sql := captureSQL(t, db, func(tx *gorm.DB) *gorm.DB {
		return repo.QueryApply(context.Background(), q).Model(&SQLTestEntity{})
	})
	n := normalize(sql)
	assert.Contains(t, n, "EXISTS(SELECT FROM unnest(tags)")
	assert.Contains(t, n, "LIKE ANY")
}

func TestSQL_FilterMultipleAreAndCombined(t *testing.T) {
	repo, db, _ := newMockRepo(t)
	q := query.New(
		query.FilterBy(filter.OpEq, "status", "active"),
		query.FilterBy(filter.OpGT, "age", 18),
	)
	sql := captureSQL(t, db, func(tx *gorm.DB) *gorm.DB {
		return repo.QueryApply(context.Background(), q).Model(&SQLTestEntity{})
	})
	n := normalize(sql)
	assert.Contains(t, n, "WHERE age > 18 AND status = 'active'")
}

func TestSQL_SortAsc(t *testing.T) {
	repo, db, _ := newMockRepo(t)
	q := query.New(query.SortBy("createdAt", query.SortAsc))
	sql := captureSQL(t, db, func(tx *gorm.DB) *gorm.DB {
		return repo.QueryApply(context.Background(), q).Model(&SQLTestEntity{})
	})
	assert.Contains(t, normalize(sql), "ORDER BY created_at ASC")
}

func TestSQL_SortDescAndCompound(t *testing.T) {
	repo, db, _ := newMockRepo(t)
	q := query.New(query.SortBy("createdAt", query.SortDesc, "name", query.SortAsc))
	sql := captureSQL(t, db, func(tx *gorm.DB) *gorm.DB {
		return repo.QueryApply(context.Background(), q).Model(&SQLTestEntity{})
	})
	assert.Contains(t, normalize(sql), "ORDER BY created_at DESC,name ASC")
}

func TestSQL_PaginationOffset(t *testing.T) {
	repo, db, _ := newMockRepo(t)
	q := query.New(query.Pagination(50, 100))
	sql := captureSQL(t, db, func(tx *gorm.DB) *gorm.DB {
		return repo.QueryApply(context.Background(), q).Model(&SQLTestEntity{})
	})
	n := normalize(sql)
	assert.Contains(t, n, "LIMIT 50")
	assert.Contains(t, n, "OFFSET 100")
}

func TestSQL_PaginationCursorAfter(t *testing.T) {
	repo, db, _ := newMockRepo(t)
	q := query.New(query.CursorPagination("", "cursor-abc", 20))
	sql := captureSQL(t, db, func(tx *gorm.DB) *gorm.DB {
		return repo.QueryApply(context.Background(), q).Model(&SQLTestEntity{})
	})
	n := normalize(sql)
	assert.Contains(t, n, "id > 'cursor-abc'")
	assert.Contains(t, n, "LIMIT 20")
	assert.NotContains(t, n, "OFFSET")
}

func TestSQL_PaginationCursorBefore(t *testing.T) {
	repo, db, _ := newMockRepo(t)
	q := query.New(query.CursorPagination("cursor-xyz", "", 10))
	sql := captureSQL(t, db, func(tx *gorm.DB) *gorm.DB {
		return repo.QueryApply(context.Background(), q).Model(&SQLTestEntity{})
	})
	n := normalize(sql)
	assert.Contains(t, n, "id < 'cursor-xyz'")
	assert.Contains(t, n, "LIMIT 10")
}

func TestSQL_SparseFieldsetsRestrictsSelect(t *testing.T) {
	repo, db, _ := newMockRepo(t)
	q := query.New(query.Fields("events", "name", "amount"))
	sql := captureSQL(t, db, func(tx *gorm.DB) *gorm.DB {
		return repo.QueryApply(context.Background(), q, pgrepo.WithResourceType("events")).Model(&SQLTestEntity{})
	})
	n := normalize(sql)
	assert.Regexp(t, regexp.MustCompile(`SELECT id, name, amount FROM`), n)
	assert.NotContains(t, n, "category")
}

func TestSQL_SparseFieldsetsForUnknownTypeIsNoOp(t *testing.T) {
	repo, db, _ := newMockRepo(t)
	q := query.New(query.Fields("articles", "title"))
	sql := captureSQL(t, db, func(tx *gorm.DB) *gorm.DB {
		return repo.QueryApply(context.Background(), q, pgrepo.WithResourceType("events")).Model(&SQLTestEntity{})
	})
	assert.Contains(t, normalize(sql), "SELECT *")
}

func TestSQL_CombinedQuery(t *testing.T) {
	repo, db, _ := newMockRepo(t)
	q := query.New(
		query.FilterBy(filter.OpEq, "status", "active"),
		query.FilterBy(filter.OpGT, "age", 18),
		query.SortBy("createdAt", query.SortDesc),
		query.Pagination(25, 50),
	)
	sql := captureSQL(t, db, func(tx *gorm.DB) *gorm.DB {
		return repo.QueryApply(context.Background(), q).Model(&SQLTestEntity{})
	})
	n := normalize(sql)
	assert.Contains(t, n, "age > 18")
	assert.Contains(t, n, "status = 'active'")
	assert.Contains(t, n, "AND")
	assert.Contains(t, n, "ORDER BY created_at DESC")
	assert.Contains(t, n, "LIMIT 25")
	assert.Contains(t, n, "OFFSET 50")
}

func TestSQL_AggregateSimpleCount(t *testing.T) {
	repo, _, _ := newMockRepo(t)
	q := query.New(query.Aggregate("total", query.AggCount, "*"))
	assert.Contains(t, normalize(captureAggSQL(t, repo, q)), `COUNT(*) AS "total"`)
}

func TestSQL_AggregateGroupBy(t *testing.T) {
	repo, _, _ := newMockRepo(t)
	q := query.New(
		query.Group("category", "region"),
		query.Aggregate("totalAmount", query.AggSum, "amount"),
		query.Aggregate("count", query.AggCount, "*"),
	)
	n := normalize(captureAggSQL(t, repo, q))
	assert.Contains(t, n, "SELECT events.category, events.region")
	assert.Contains(t, n, `SUM(events.amount) AS "totalAmount"`)
	assert.Contains(t, n, `COUNT(*) AS "count"`)
	assert.Contains(t, n, "GROUP BY events.category, events.region")
}

func captureAggSQL(t *testing.T, repo *pgrepo.Repo, q query.Query) string {
	t.Helper()
	var out []SQLTestEntity
	tx := repo.AggregateApply(context.Background(), &SQLTestEntity{}, q, "events").Find(&out)
	return tx.Dialector.Explain(tx.Statement.SQL.String(), tx.Statement.Vars...)
}

func TestSQL_AggregateBucketHourly(t *testing.T) {
	repo, _, _ := newMockRepo(t)
	q := query.New(
		query.Bucket(time.Hour),
		query.Group("category"),
		query.Aggregate("count", query.AggCount, "*"),
	)
	n := normalize(captureAggSQL(t, repo, q))
	assert.Contains(t, n, `date_trunc('hour', events.created_at) AS _bucket`)
	assert.Contains(t, n, "GROUP BY _bucket, events.category")
}

func TestSQL_AggregateBucketDaily(t *testing.T) {
	repo, _, _ := newMockRepo(t)
	q := query.New(query.Bucket(24*time.Hour), query.Aggregate("count", query.AggCount, "*"))
	assert.Contains(t, normalize(captureAggSQL(t, repo, q)), `date_trunc('day', events.created_at) AS _bucket`)
}

func TestSQL_AggregateAllOps(t *testing.T) {
	cases := []struct {
		op   query.AggOp
		want string
	}{
		{query.AggCount, "COUNT"},
		{query.AggSum, "SUM"},
		{query.AggAvg, "AVG"},
		{query.AggMin, "MIN"},
		{query.AggMax, "MAX"},
	}
	for _, c := range cases {
		repo, _, _ := newMockRepo(t)
		q := query.New(query.Aggregate("m", c.op, "amount"))
		assert.Contains(t, normalize(captureAggSQL(t, repo, q)), c.want+`(events.amount) AS "m"`, "op=%v", c.op)
	}
}

func TestSQL_AggregateWithFilterAndSort(t *testing.T) {
	repo, _, _ := newMockRepo(t)
	q := query.New(
		query.FilterBy(filter.OpEq, "status", "success"),
		query.Group("category"),
		query.Aggregate("totalAmount", query.AggSum, "amount"),
		query.SortBy("totalAmount", query.SortDesc),
		query.Pagination(10, 0),
	)
	n := normalize(captureAggSQL(t, repo, q))
	assert.Contains(t, n, `SUM(events.amount) AS "totalAmount"`)
	assert.Contains(t, n, "WHERE events.status = 'success'")
	// GORM quotes single-column GROUP BY: `"events"."category"`; just
	// assert the column name appears after GROUP BY.
	assert.Regexp(t, regexp.MustCompile(`GROUP BY [\w."]*category[\w."]*`), n)
	assert.Contains(t, n, "ORDER BY totalAmount DESC")
	assert.Contains(t, n, "LIMIT 10")
}

func TestSQL_OrGroupSingleGroup(t *testing.T) {
	repo, db, _ := newMockRepo(t)
	q := query.New(
		query.OrGroup(
			query.FilterBy(filter.OpEq, "assignee", "u-1"),
			query.FilterBy(filter.OpEq, "createdBy", "u-1"),
		),
	)
	sql := captureSQL(t, db, func(tx *gorm.DB) *gorm.DB {
		return repo.QueryApply(context.Background(), q).Model(&SQLTestEntity{})
	})
	n := normalize(sql)
	assert.Contains(t, n, "(assignee = 'u-1' AND created_by = 'u-1')")
}

func TestSQL_OrGroupTwoGroupsDNF(t *testing.T) {
	repo, db, _ := newMockRepo(t)
	q := query.New(
		query.OrGroup(
			query.FilterBy(filter.OpEq, "status", "active"),
			query.FilterBy(filter.OpEq, "amount", 100),
		),
		query.OrGroup(
			query.FilterBy(filter.OpEq, "status", "archived"),
			query.FilterBy(filter.OpGT, "amount", 1000),
		),
	)
	sql := captureSQL(t, db, func(tx *gorm.DB) *gorm.DB {
		return repo.QueryApply(context.Background(), q).Model(&SQLTestEntity{})
	})
	n := normalize(sql)
	assert.Contains(t, n, "((amount = 100 AND status = 'active') OR (amount > 1000 AND status = 'archived'))")
}

func TestSQL_OrGroupMixedWithTopLevel(t *testing.T) {
	repo, db, _ := newMockRepo(t)
	q := query.New(
		query.FilterBy(filter.OpEq, "status", "active"),
		query.OrGroup(query.FilterBy(filter.OpEq, "assignee", "u-1")),
		query.OrGroup(query.FilterBy(filter.OpEq, "createdBy", "u-1")),
	)
	sql := captureSQL(t, db, func(tx *gorm.DB) *gorm.DB {
		return repo.QueryApply(context.Background(), q).Model(&SQLTestEntity{})
	})
	n := normalize(sql)
	assert.Contains(t, n, "status = 'active'")
	assert.Contains(t, n, "((assignee = 'u-1') OR (created_by = 'u-1'))")
}

func TestSQL_OrGroupSingleGroupNoTopLevel(t *testing.T) {
	repo, db, _ := newMockRepo(t)
	q := query.New(query.OrGroup(query.FilterBy(filter.OpEq, "assignee", "u-1")))
	sql := captureSQL(t, db, func(tx *gorm.DB) *gorm.DB {
		return repo.QueryApply(context.Background(), q).Model(&SQLTestEntity{})
	})
	n := normalize(sql)
	assert.Contains(t, n, "(assignee = 'u-1')")
	assert.NotContains(t, n, " OR ")
}

func TestSQL_OrGroupWithOperatorVariety(t *testing.T) {
	repo, db, _ := newMockRepo(t)
	q := query.New(
		query.OrGroup(
			query.FilterBy(filter.OpLike, "name", "alice"),
			query.FilterBy(filter.OpIn, "status", []string{"active", "pending"}),
		),
		query.OrGroup(query.FilterBy(filter.OpIsNull, "deletedAt", nil)),
	)
	sql := captureSQL(t, db, func(tx *gorm.DB) *gorm.DB {
		return repo.QueryApply(context.Background(), q).Model(&SQLTestEntity{})
	})
	n := normalize(sql)
	assert.Contains(t, n, "name LIKE '%alice%'")
	assert.Contains(t, n, "status IN ('active','pending')")
	assert.Contains(t, n, "deleted_at IS NULL")
	assert.Contains(t, n, " OR ")
}

func TestSQL_CountApplyAppliesFilters(t *testing.T) {
	repo, _, _ := newMockRepo(t)
	q := query.New(query.FilterBy(filter.OpEq, "status", "active"))
	var count int64
	tx := repo.CountApply(context.Background(), &SQLTestEntity{}, q).Count(&count)
	sql := tx.Dialector.Explain(tx.Statement.SQL.String(), tx.Statement.Vars...)
	n := normalize(sql)
	assert.Contains(t, n, `SELECT count(*) FROM "events"`)
	assert.Contains(t, n, "status = 'active'")
}

var _ = sqlmock.QueryMatcherEqual
var _ = driver.RowsAffected(0)
