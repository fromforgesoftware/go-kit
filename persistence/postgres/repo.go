package postgres

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"golang.org/x/exp/maps"
	"gorm.io/gorm"

	"github.com/lib/pq"

	apierrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/filter"
	"github.com/fromforgesoftware/go-kit/persistence/gormdb"
	"github.com/fromforgesoftware/go-kit/search/query"
	"github.com/fromforgesoftware/go-kit/slicesx"
)

type Repo struct {
	DB      *gormdb.DBClient
	fMapper map[string]string
}

func NewRepo(db *gormdb.DBClient, fMapper map[string]string) (*Repo, error) {
	if db == nil {
		return nil, errors.New("missing db client")
	}
	if fMapper == nil {
		return nil, errors.New("missing field map")
	}
	fieldMapper := make(map[string]string, len(fMapper))
	for k, v := range fMapper {
		fieldMapper[k] = v
	}

	return &Repo{
		DB:      db,
		fMapper: fieldMapper,
	}, nil
}

func (r *Repo) FMapper() map[string]string {
	return r.fMapper
}

func (r *Repo) Commit() error {
	return r.DB.Commit().Error
}

func (r *Repo) Rollback() error {
	return r.DB.Rollback().Error
}

func (r *Repo) QueryApply(ctx context.Context, q query.Query, ops ...queryApplyOption) (tx *gorm.DB) {
	return r.queryApply(ctx, q, "", ops...)
}

func (r *Repo) QueryApplyWithTableName(ctx context.Context, q query.Query, tableName string, ops ...queryApplyOption) (tx *gorm.DB) {
	return r.queryApply(ctx, q, tableName, ops...)
}

func (r *Repo) CountApply(ctx context.Context, model any, q query.Query) (tx *gorm.DB) {
	return r.countApply(ctx, model, q, "")
}

func (r *Repo) CountApplyWithTableName(ctx context.Context, model any, q query.Query, tableName string) (tx *gorm.DB) {
	return r.countApply(ctx, model, q, tableName)
}

func (r *Repo) PatchApply(ctx context.Context, q query.Query, model any, toPatch map[string]any) (tx *gorm.DB) {
	mapped := make(map[string]any, len(toPatch))
	for k, v := range toPatch {
		mappedKey, ok := r.fMapper[k]
		if !ok {
			mappedKey = k
		}
		mapped[mappedKey] = v
	}

	return r.
		queryApply(ctx, q, "").
		Model(model).
		Updates(mapped)
}

func (r *Repo) queryApply(ctx context.Context, q query.Query, tableName string, ops ...queryApplyOption) (tx *gorm.DB) {
	ops = append(ops, withLock(ctx, tableName))

	s := new(queryApplySetup)
	for _, v := range ops {
		v(s)
	}

	tx = r.DB.WithContext(ctx)
	if q == nil {
		return
	}
	tx = r.fieldsApply(tx, q.Fields(), tableName, s.resourceType)
	tx = r.filterApply(tx, q.Filters(), tableName)
	tx = r.orGroupsApply(tx, q.OrGroups(), tableName)
	tx = r.cursorApply(tx, q.Pagination(), tableName)
	tx = r.sortingApply(tx, q.Sorting())
	if p := q.Pagination(); p != nil {
		tx = r.paginationApply(tx, p)
	}
	if s.lock != nil {
		tx = tx.Clauses(s.lock)
	}

	return
}

func (r *Repo) AggregateApply(ctx context.Context, model any, q query.Query, tableName string) *gorm.DB {
	tx := r.DB.WithContext(ctx).Model(model)
	if tableName != "" {
		tx = tx.Table(tableName)
	}
	if q == nil {
		return tx
	}

	selects := []string{}
	if d := q.Bucket(); d > 0 {
		col := r.mappedCol("createdAt", tableName)
		selects = append(selects, fmt.Sprintf("date_trunc('%s', %s) AS _bucket", bucketUnitName(d), col))
	}
	for _, g := range q.Group() {
		selects = append(selects, r.mappedCol(g, tableName))
	}
	for _, a := range q.Aggregations() {
		col := "*"
		if a.Field != "*" && a.Field != "" {
			col = r.mappedCol(a.Field, tableName)
		}
		selects = append(selects, fmt.Sprintf(`%s(%s) AS "%s"`, a.Operator.String(), col, a.Alias))
	}
	if len(selects) > 0 {
		tx = tx.Select(strings.Join(selects, ", "))
	}

	tx = r.filterApply(tx, q.Filters(), tableName)
	tx = r.orGroupsApply(tx, q.OrGroups(), tableName)

	groupCols := []string{}
	if q.Bucket() > 0 {
		groupCols = append(groupCols, "_bucket")
	}
	for _, g := range q.Group() {
		groupCols = append(groupCols, r.mappedCol(g, tableName))
	}
	if len(groupCols) > 0 {
		tx = tx.Group(strings.Join(groupCols, ", "))
	}

	tx = r.sortingApply(tx, q.Sorting())
	if p := q.Pagination(); p != nil {
		tx = r.paginationApply(tx, p)
	}
	return tx
}

func (r *Repo) mappedCol(name, tableName string) string {
	col := r.fMapper[name]
	if col == "" {
		col = name
	}
	if tableName != "" && !strings.Contains(col, ".") {
		col = tableName + "." + col
	}
	return col
}

func bucketUnitName(d time.Duration) string {
	switch {
	case d >= 24*time.Hour && d%(24*time.Hour) == 0:
		return "day"
	case d >= time.Hour && d%time.Hour == 0:
		return "hour"
	case d >= time.Minute && d%time.Minute == 0:
		return "minute"
	default:
		return "second"
	}
}

func (r *Repo) countApply(ctx context.Context, model any, q query.Query, tableName string) (tx *gorm.DB) {
	tx = r.DB.WithContext(ctx).Model(model)
	if q == nil {
		return
	}
	if tableName != "" {
		tx = tx.Table(tableName)
	}
	tx = r.filterApply(tx, q.Filters(), tableName)
	tx = r.orGroupsApply(tx, q.OrGroups(), tableName)

	return
}

func (r *Repo) filterApply(tx *gorm.DB, filters query.Filters[any], tableName string) *gorm.DB {
	if len(filters) < 1 {
		return tx
	}
	sql, args := r.filterClauseAnd(filters, tableName)
	return tx.Where(sql, args...)
}

func (r *Repo) orGroupsApply(tx *gorm.DB, groups []query.Filters[any], tableName string) *gorm.DB {
	if len(groups) == 0 {
		return tx
	}
	parts := []string{}
	args := []any{}
	for _, g := range groups {
		if len(g) == 0 {
			continue
		}
		inner, innerArgs := r.filterClauseAnd(g, tableName)
		parts = append(parts, "("+inner+")")
		args = append(args, innerArgs...)
	}
	if len(parts) == 0 {
		return tx
	}
	if len(parts) == 1 {
		return tx.Where(parts[0], args...)
	}
	return tx.Where("("+strings.Join(parts, " OR ")+")", args...)
}

func (r *Repo) filterClauseAnd(filters query.Filters[any], tableName string) (string, []any) {
	sqlQuery := strings.Builder{}
	args := []any{}
	keys := maps.Keys(filters)
	sort.Strings(keys)
	for i, key := range keys {
		filt := filters[key]
		colName := r.fMapper[key]
		if colName == "" {
			colName = key
		}
		if tableName != "" && !strings.Contains(colName, ".") {
			colName = tableName + "." + colName
		}
		switch filt.Operator() {
		case filter.OpIsNull, filter.OpNotNull:
			if filt.Value() == nil {
				fmt.Fprintf(&sqlQuery, "%s %s NULL", colName, filt.Operator().String())
			} else {
				sqlQuery.WriteString(simpleArg(colName, filt.Operator()))
				args = append(args, filt.Value())
			}
		case filter.OpLike, filter.OpNotLike:
			sqlQuery.WriteString(simpleArg(colName, filt.Operator()))
			args = append(args, fmt.Sprintf("%%%v%%", filt.Value()))
		case filter.OpIn, filter.OpNotIn, filter.OpContainsLike:
			vals, ok := filt.Value().([]string)
			if !ok {
				inputVal := reflect.ValueOf(filt.Value())
				if inputVal.Kind() == reflect.Slice {
					output := make([]string, inputVal.Len())
					for i := 0; i < inputVal.Len(); i++ {
						output[i] = fmt.Sprintf("%v", inputVal.Index(i).Interface())
					}
					vals = output
				}
			}
			sqlQuery.WriteString(sliceArg(filt.Operator(), colName, vals))
			args = append(args, slicesx.Map(vals, func(s string) any { return s })...)
		case filter.OpContains:
			sqlQuery.WriteString(simpleArg(colName, filt.Operator()))
			if kind := reflect.ValueOf(filt.Value()).Kind(); kind == reflect.Slice || kind == reflect.Array {
				args = append(args, pq.Array(filt.Value()))
			} else {
				args = append(args, pq.Array([]any{filt.Value()}))
			}
		case filter.OpBetween:
			vals := btwArgs(filt.Value())
			fmt.Fprintf(&sqlQuery, "%s %s ? AND ?", colName, filt.Operator())
			args = append(args, vals...)
		default:
			sqlQuery.WriteString(simpleArg(colName, filt.Operator()))
			if kind := reflect.ValueOf(filt.Value()).Kind(); kind == reflect.Slice || kind == reflect.Array {
				args = append(args, pq.Array(filt.Value()))
			} else {
				args = append(args, filt.Value())
			}
		}
		if i < len(filters)-1 {
			sqlQuery.WriteString(" AND ")
		}
	}
	return sqlQuery.String(), args
}

func (r *Repo) sortingApply(tx *gorm.DB, sorting *query.SortingParams) *gorm.DB {
	if sorting == nil {
		return tx
	}
	keys := sorting.Keys()
	if len(keys) < 1 {
		return tx
	}
	allParams := make([]string, len(keys))
	idx := 0
	for _, key := range keys {
		dir := sorting.Get(key)
		col := r.fMapper[key]
		if col == "" {
			col = key
		}
		allParams[idx] = fmt.Sprintf("%s %s", col, dir)
		idx++
	}
	return tx.Order(strings.Join(allParams, ","))
}

func (r *Repo) paginationApply(tx *gorm.DB, p *query.PaginationParams) *gorm.DB {
	if p.IsCursor() {
		if p.Size > 0 {
			tx = tx.Limit(p.Size)
		}
		return tx
	}
	tx = tx.Offset(p.Offset)
	if p.Limit > 0 {
		tx = tx.Limit(p.Limit)
	}
	return tx
}

func (r *Repo) cursorApply(tx *gorm.DB, p *query.PaginationParams, tableName string) *gorm.DB {
	if p == nil || !p.IsCursor() {
		return tx
	}
	col := r.mappedCol("id", tableName)
	if p.After != "" {
		tx = tx.Where(col+" > ?", p.After)
	}
	if p.Before != "" {
		tx = tx.Where(col+" < ?", p.Before)
	}
	return tx
}

func (r *Repo) fieldsApply(tx *gorm.DB, fields query.SparseFieldsets, tableName, resourceType string) *gorm.DB {
	if len(fields) == 0 || resourceType == "" {
		return tx
	}
	names, ok := fields[resourceType]
	if !ok || len(names) == 0 {
		return tx
	}
	cols := []string{r.mappedCol("id", tableName)}
	for _, n := range names {
		col := r.mappedCol(n, tableName)
		if col != cols[0] {
			cols = append(cols, col)
		}
	}
	return tx.Select(strings.Join(cols, ", "))
}

// -----------------------------------------------------------------------------
// Helpers: Usage
// -----------------------------------------------------------------------------

func filterOp(op filter.Operator) string {
	switch op {
	case filter.OpEq:
		return "="
	case filter.OpNEq:
		return "<>"
	case filter.OpGT:
		return ">"
	case filter.OpGTEq:
		return ">="
	case filter.OpLT:
		return "<"
	case filter.OpLTEq:
		return "<="
	case filter.OpIn:
		return "IN"
	case filter.OpNotIn:
		return "NOT IN"
	case filter.OpLike:
		return "LIKE"
	case filter.OpNotLike:
		return "NOT LIKE"
	case filter.OpContainsLike:
		return "LIKE ANY"
	case filter.OpBetween:
		return "BETWEEN"
	case filter.OpContains:
		return "@>"
	case filter.OpIsNull:
		return "IS"
	case filter.OpNotNull:
		return "IS NOT"
	default:
		panic(apierrors.InternalError(fmt.Sprintf("operator %s is not supported", op)))
	}
}

func simpleArg(colName string, operator filter.Operator) string {
	return fmt.Sprintf("%s %s ?", colName, filterOp(operator))
}

func sliceArg(operator filter.Operator, colName string, vals []string) string {
	subquery := strings.Builder{}
	if operator == filter.OpContainsLike {
		for i := 0; i < len(vals); i++ {
			subquery.WriteString("'%' || ? || '%'")
			if i < len(vals)-1 {
				subquery.WriteRune(',')
			}
		}
		return fmt.Sprintf(
			"EXISTS(SELECT FROM unnest(%s) cl_alias WHERE cl_alias %s(ARRAY[%s]))",
			colName,
			filterOp(operator),
			subquery.String(),
		)
	}
	for i := 0; i < len(vals); i++ {
		subquery.WriteString("?")
		if i < len(vals)-1 {
			subquery.WriteRune(',')
		}
	}

	return fmt.Sprintf(
		"%s %s (%s)",
		colName,
		filterOp(operator),
		subquery.String(),
	)
}

func btwArgs(a any) []any {
	args := make([]any, 2)

	val := reflect.ValueOf(a)
	if val.Kind() != reflect.Slice {
		return args
	}

	// Generic handling
	for i := 0; i < val.Len() && i < 2; i++ {
		args[i] = val.Index(i).Interface()
	}
	return args
}
