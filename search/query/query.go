// Package query is the search Query DSL — filter operators, sort
// expressions, sparse fieldsets, includes, and pagination styles —
// shared by every REST + gRPC list endpoint in go/kit.
package query

import (
	"fmt"
	"reflect"
	"slices"
	"time"

	"github.com/fromforgesoftware/go-kit/filter"
)

const (
	FieldNameQuery      = "query"
	FieldNamePagination = "pagination"
	FieldNameSorting    = "sorting"
	FieldNameIncludes   = "includes"
)

type SortingDir uint

const (
	SortDirUndefined SortingDir = iota
	SortAsc
	SortDesc
)

func (sd SortingDir) Valid() bool {
	return sd == SortAsc || sd == SortDesc
}

func (sd SortingDir) String() string {
	switch sd {
	case SortAsc:
		return "ASC"
	case SortDesc:
		return "DESC"
	case SortDirUndefined:
		return ""
	default:
		return ""
	}
}

type SortingParams struct {
	m    map[string]SortingDir
	keys []string
}

func newSortingParams() *SortingParams {
	return &SortingParams{
		m:    make(map[string]SortingDir),
		keys: make([]string, 0),
	}
}

func (sp *SortingParams) Set(key string, v SortingDir) {
	_, present := sp.m[key]
	sp.m[key] = v
	if !present {
		sp.keys = append(sp.keys, key)
	}
}

func (sp *SortingParams) Keys() []string {
	return sp.keys
}

func (sp *SortingParams) Get(key string) SortingDir {
	value, present := sp.m[key]
	if !present {
		return SortAsc
	}

	return value
}

type PaginationParams struct {
	Limit  int
	Offset int
	Before string
	After  string
	Size   int
}

func (p *PaginationParams) Delete() {
	if p != nil {
		p.Limit = 0
		p.Offset = 0
		p.Before = ""
		p.After = ""
		p.Size = 0
	}
}

func (p *PaginationParams) IsCursor() bool {
	return p != nil && (p.Before != "" || p.After != "")
}

type Filters[T any] map[string]filter.FieldFilter[T]

func (qf Filters[T]) Get(key string) filter.FieldFilter[T] {
	return qf[key]
}

func (qf Filters[T]) Exists(keys ...string) bool {
	if len(keys) < 1 {
		panic("exists called without any keys")
	}
	for _, k := range keys {
		if qf[k] == nil {
			return false
		}
	}
	return true
}

func (qf Filters[T]) Delete(key string) {
	if qf.Exists(key) {
		delete(qf, key)
	}
}

func GetFilterVal[T any](fName string, filters Filters[any]) T {
	f := filters.Get(fName)
	var fVal T
	if f != nil {
		fVal = f.Value().(T)
	}
	return fVal
}

func GetFilterValOrDefault[T any](fName string, filters Filters[any], def T) T {
	f := filters.Get(fName)
	if f != nil {
		return f.Value().(T)
	}
	return def
}

func GetFilterSingleOrArrayVal[T any](fName string, filters Filters[any]) []T {
	f := filters.Get(fName)
	if f == nil {
		return []T{}
	}

	switch f.Operator() {
	case filter.OpIn:
		if arrayVal, ok := f.Value().([]T); ok {
			return arrayVal
		}
		fallthrough
	case filter.OpEq:
		if singleVal, ok := f.Value().(T); ok {
			return []T{singleVal}
		}
		fallthrough
	default:
		return []T{}
	}
}

func DoesInclude(q Query, relationship string) bool {
	return slices.Contains(q.IncludedResourceObjects(), relationship)
}

func AddFilter(q Query, operator filter.Operator, name string, val any) {
	if q.Filters().Exists(name) {
		return
	}
	q.Filters()[name] = filter.NewFieldFilter(operator, name, val)
}

func UpdateFilter[T any](q Query, name string, updateFunc func(filter.Operator, T) (filter.Operator, string, any)) {
	if !q.Filters().Exists(name) {
		return
	}
	f := q.Filters()[name]
	v, ok := f.Value().(T)
	if !ok {
		return
	}
	q.Filters()[name] = filter.NewFieldFilter(updateFunc(f.Operator(), v))
}

type SparseFieldsets map[string][]string

type AggOp uint

const (
	AggUndefined AggOp = iota
	AggCount
	AggSum
	AggAvg
	AggMin
	AggMax
)

func (op AggOp) Valid() bool { return op > AggUndefined && op <= AggMax }

func (op AggOp) String() string {
	switch op {
	case AggCount:
		return "COUNT"
	case AggSum:
		return "SUM"
	case AggAvg:
		return "AVG"
	case AggMin:
		return "MIN"
	case AggMax:
		return "MAX"
	}
	return ""
}

type Aggregation struct {
	Alias    string
	Operator AggOp
	Field    string
}

type Query interface {
	Filters() Filters[any]
	OrGroups() []Filters[any]
	Sorting() *SortingParams
	Merge(q Query)
	Pagination() *PaginationParams
	IncludedResourceObjects() []string
	Fields() SparseFieldsets
	Group() []string
	Aggregations() []Aggregation
	Bucket() time.Duration
	Equal(another Query) bool
}

type Option func(q *query)

func sortBy(key string, dir SortingDir) Option {
	return func(q *query) {
		if len(key) < 1 {
			return
		}
		if !dir.Valid() {
			return
		}
		q.sortingParams.Set(key, dir)
	}
}

func SortBy(sortParams ...any) Option {
	return func(q *query) {
		for i := 0; i < len(sortParams); i += 2 {
			if i+1 >= len(sortParams) {
				break
			}

			key, ok := sortParams[i].(string)
			if !ok {
				fmtKey, keyCast := sortParams[i].(fmt.Stringer)
				if !keyCast || fmtKey == nil {
					continue
				}
				key = fmtKey.String()
			}
			dir, dirCast := sortParams[i+1].(SortingDir)
			if !dirCast {
				continue
			}

			sortBy(key, dir)(q)
		}
	}
}

func Filter(f filter.FieldFilter[any]) Option {
	return func(q *query) {
		if f == nil {
			return
		}
		q.filters[f.Name()] = f
	}
}

func FilterBy(op filter.Operator, fieldName, val any) Option {
	return func(q *query) {
		if !op.Valid() {
			return
		}
		name, nameCast := fieldName.(string)
		if !nameCast {
			fmtName, nameCast := fieldName.(fmt.Stringer)
			if !nameCast || fmtName == nil {
				return
			}
			name = fmtName.String()
		}
		if len(name) < 1 {
			return
		}
		if val == nil && op != filter.OpIsNull && op != filter.OpNotNull {
			return
		}
		q.filters[name] = filter.NewFieldFilter(op, name, val)
	}
}

func FilterByTriples(opFieldVals ...any) Option {
	return func(q *query) {
		for i := 0; i < len(opFieldVals); i += 3 {
			if i+2 >= len(opFieldVals) {
				break
			}

			op, opCast := opFieldVals[i].(filter.Operator)
			if !opCast {
				continue
			}

			FilterBy(op, opFieldVals[i+1], opFieldVals[i+2])(q)
		}
	}
}

func Pagination(limit, offset int) Option {
	return func(q *query) {
		q.pagination = &PaginationParams{
			Limit:  limit,
			Offset: offset,
		}
	}
}

func CursorPagination(before, after string, size int) Option {
	return func(q *query) {
		q.pagination = &PaginationParams{
			Before: before,
			After:  after,
			Size:   size,
		}
	}
}

func IncludedResourceObjects(relationshipNames ...string) Option {
	return func(q *query) {
		q.includedResourceObjects = append(q.includedResourceObjects, relationshipNames...)
	}
}

func Fields(resourceType string, names ...string) Option {
	return func(q *query) {
		if resourceType == "" || len(names) == 0 {
			return
		}
		if q.fields == nil {
			q.fields = make(SparseFieldsets)
		}
		q.fields[resourceType] = append([]string(nil), names...)
	}
}

func Group(dimensions ...string) Option {
	return func(q *query) {
		for _, d := range dimensions {
			if d == "" {
				continue
			}
			q.group = append(q.group, d)
		}
	}
}

func Aggregate(alias string, op AggOp, field string) Option {
	return func(q *query) {
		if alias == "" || !op.Valid() {
			return
		}
		if field == "" {
			field = "*"
		}
		q.aggs = append(q.aggs, Aggregation{Alias: alias, Operator: op, Field: field})
	}
}

func Bucket(d time.Duration) Option {
	return func(q *query) {
		if d > 0 {
			q.bucket = d
		}
	}
}

func OrGroup(opts ...Option) Option {
	return func(q *query) {
		sub := newEmptyQuery()
		for _, opt := range opts {
			opt(sub)
		}
		if len(sub.filters) == 0 {
			return
		}
		q.orGroups = append(q.orGroups, Filters[any](sub.filters))
	}
}

func newEmptyQuery() *query {
	return &query{
		filters:       make(map[string]filter.FieldFilter[any]),
		sortingParams: newSortingParams(),
	}
}

type query struct {
	filters                 map[string]filter.FieldFilter[any]
	orGroups                []Filters[any]
	sortingParams           *SortingParams
	pagination              *PaginationParams
	includedResourceObjects []string
	fields                  SparseFieldsets
	group                   []string
	aggs                    []Aggregation
	bucket                  time.Duration
}

func (q *query) Filters() Filters[any] {
	return q.filters
}

func (q *query) Sorting() *SortingParams {
	return q.sortingParams
}

func (q *query) Merge(m Query) {
	if m != nil {
		q.mergeFilters(m.Filters())
		q.mergeSorting(m.Sorting())
		if m.Pagination() != nil {
			q.pagination = m.Pagination()
		}
		if m.IncludedResourceObjects() != nil {
			q.includedResourceObjects = m.IncludedResourceObjects()
		}
		q.mergeFields(m.Fields())
		if og := m.OrGroups(); len(og) > 0 {
			q.orGroups = append([]Filters[any](nil), og...)
		}
		if g := m.Group(); len(g) > 0 {
			q.group = append([]string(nil), g...)
		}
		if a := m.Aggregations(); len(a) > 0 {
			q.aggs = append([]Aggregation(nil), a...)
		}
		if b := m.Bucket(); b > 0 {
			q.bucket = b
		}
	}
}

func (q *query) Group() []string             { return q.group }
func (q *query) Aggregations() []Aggregation { return q.aggs }
func (q *query) Bucket() time.Duration       { return q.bucket }
func (q *query) OrGroups() []Filters[any]    { return q.orGroups }

func (q *query) Fields() SparseFieldsets {
	return q.fields
}

func (q *query) mergeFields(other SparseFieldsets) {
	if len(other) == 0 {
		return
	}
	if q.fields == nil {
		q.fields = make(SparseFieldsets, len(other))
	}
	for resourceType, names := range other {
		if resourceType == "" || len(names) == 0 {
			continue
		}
		q.fields[resourceType] = append([]string(nil), names...)
	}
}

func (q *query) Pagination() *PaginationParams {
	return q.pagination
}

func (q *query) IncludedResourceObjects() []string {
	return q.includedResourceObjects
}

func (q *query) mergeFilters(filters Filters[any]) {
	for _, f := range filters {
		if f.Value() == nil && f.Operator() != filter.OpIsNull && f.Operator() != filter.OpNotNull {
			continue
		}
		q.filters[f.Name()] = f
	}
}

func (q *query) mergeSorting(sortParams *SortingParams) {
	for _, key := range sortParams.Keys() {
		if len(key) < 1 {
			continue
		}
		dir := sortParams.Get(key)
		if len(key) < 1 || !dir.Valid() {
			continue
		}
		q.sortingParams.Set(key, dir)
	}
}

func (q *query) Equal(another Query) bool {
	if (q == nil && another != nil) ||
		q != nil && another == nil {
		return false
	}
	if q == nil && another == nil {
		return true
	}

	return reflect.DeepEqual(q, another.(*query))
}

func New(opts ...Option) Query {
	q := &query{
		filters:       make(map[string]filter.FieldFilter[any]),
		sortingParams: newSortingParams(),
	}

	for _, opt := range opts {
		opt(q)
	}

	return q
}
