package query

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	kiterrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/filter"
)

const (
	defaultPagLimit  = 50
	defaultPagOffset = 0

	filterSplits = 3
)

var (
	ErrInvalidFilterFormat = errors.New("filter format should be filter[field][operator]")
	ErrInvalidOperator     = errors.New("invalid operator")

	//nolint:gochecknoglobals // map is used in every GET request with filters, it's more efficient to keep it global
	Operators = map[string]filter.Operator{
		"eq":       filter.OpEq,
		"ne":       filter.OpNEq,
		"gt":       filter.OpGT,
		"gte":      filter.OpGTEq,
		"lt":       filter.OpLT,
		"lte":      filter.OpLTEq,
		"in":       filter.OpIn,
		"not-in":   filter.OpNotIn,
		"like":     filter.OpLike,
		"not-like": filter.OpNotLike,
		"btw":      filter.OpBetween,
		"any":      filter.OpContains,
		"any-like": filter.OpContainsLike,
		"is-null":  filter.OpIsNull,
		"not-null": filter.OpNotNull,
	}

	//nolint:gochecknoglobals // map is used in every GET request with filters, it's more efficient to keep it global
	OperatorStrings = map[filter.Operator]string{
		filter.OpEq:           "eq",
		filter.OpNEq:          "ne",
		filter.OpGT:           "gt",
		filter.OpGTEq:         "gte",
		filter.OpLT:           "lt",
		filter.OpLTEq:         "lte",
		filter.OpIn:           "in",
		filter.OpNotIn:        "not-in",
		filter.OpLike:         "like",
		filter.OpNotLike:      "not-like",
		filter.OpBetween:      "btw",
		filter.OpContains:     "any",
		filter.OpContainsLike: "any-like",
		filter.OpIsNull:       "is-null",
		filter.OpNotNull:      "not-null",
	}
)

func ParseOperator(val string) filter.Operator {
	v, ok := Operators[val]
	if !ok {
		return filter.OpUndefined
	}
	return v
}

func MarshalOperator(op filter.Operator) string {
	return OperatorStrings[op]
}

func parseFilter(filterKey string) (string, filter.Operator, error) {
	split := strings.Split(filterKey, "[")
	if len(split) != filterSplits {
		return "", filter.OpUndefined, kiterrors.InvalidArgument(fmt.Sprintf("invalid filter format: %s", filterKey))
	}

	fName := strings.ReplaceAll(split[1], "]", "")
	op := ParseOperator(strings.ReplaceAll(split[2], "]", ""))
	if op == filter.OpUndefined {
		return "", filter.OpUndefined, kiterrors.InvalidArgument(fmt.Sprintf("invalid operator: %s", split[2]))
	}

	return fName, op, nil
}

func parseValue(op filter.Operator, val []string) any {
	if len(val) == 1 {

		if strings.ToLower(val[0]) == "null" {
			return nil
		}

		match, err := regexp.MatchString("^(?i)(true|false)$", val[0])
		if err != nil {
			return val[0]
		}
		if match {
			if b, err := strconv.ParseBool(val[0]); err == nil {
				return b
			}
		}
		if strings.Contains(val[0], ",") {
			return strings.Split(val[0], ",")
		} else if op == filter.OpIn || op == filter.OpContainsLike {
			return []string{val[0]}
		}
		return val[0]
	}
	return val
}

func searchFromURL(uri *url.URL) ([]Option, error) {
	opts := []Option{}
	orGroups := map[int][]Option{}
	for key, values := range uri.Query() {
		if !strings.HasPrefix(key, "filter[") {
			continue
		}
		parts := strings.Split(key, "[")
		if len(parts) == 5 && parts[1] == "or]" {
			idxRaw := strings.TrimSuffix(parts[2], "]")
			idx, err := strconv.Atoi(idxRaw)
			if err != nil || idx < 0 {
				return nil, kiterrors.InvalidArgument(fmt.Sprintf("invalid OR group index %q", idxRaw))
			}
			fName := strings.TrimSuffix(parts[3], "]")
			opRaw := strings.TrimSuffix(parts[4], "]")
			op := ParseOperator(opRaw)
			if op == filter.OpUndefined {
				return nil, kiterrors.InvalidArgument(fmt.Sprintf("invalid operator in OR group: %s", opRaw))
			}
			if fName == "" {
				return nil, kiterrors.InvalidArgument("empty field name in OR group")
			}
			orGroups[idx] = append(orGroups[idx], FilterBy(op, fName, parseValue(op, values)))
			continue
		}
		fName, op, err := parseFilter(key)
		if err != nil {
			return nil, err
		}
		opts = append(opts, FilterBy(op, fName, parseValue(op, values)))
	}
	indexes := make([]int, 0, len(orGroups))
	for i := range orGroups {
		indexes = append(indexes, i)
	}
	sort.Ints(indexes)
	for _, i := range indexes {
		opts = append(opts, OrGroup(orGroups[i]...))
	}
	return opts, nil
}

func paginationFromURL(uri *url.URL, defaultIfEmpty bool) (opt Option, err error) {
	q := uri.Query()
	l := q.Get("page[limit]")
	o := q.Get("page[offset]")
	pn := q.Get("page[number]")
	ps := q.Get("page[size]")
	before := q.Get("page[before]")
	after := q.Get("page[after]")

	if l == "" && o == "" && pn == "" && ps == "" && before == "" && after == "" && !defaultIfEmpty {
		return nil, nil
	}

	if before != "" || after != "" {
		size := 0
		if ps != "" {
			parsed, err := strconv.Atoi(ps)
			if err != nil || parsed <= 0 {
				return nil, kiterrors.InvalidArgument(fmt.Sprintf("invalid page[size]: %s", ps))
			}
			size = parsed
		}
		return CursorPagination(before, after, size), nil
	}

	if pn != "" || ps != "" {
		number := 0
		size := defaultPagLimit
		if pn != "" {
			n, err := strconv.Atoi(pn)
			if err != nil || n < 0 {
				return nil, kiterrors.InvalidArgument(fmt.Sprintf("invalid page[number]: %s", pn))
			}
			number = n
		}
		if ps != "" {
			s, err := strconv.Atoi(ps)
			if err != nil || s <= 0 {
				return nil, kiterrors.InvalidArgument(fmt.Sprintf("invalid page[size]: %s", ps))
			}
			size = s
		}
		return Pagination(size, number*size), nil
	}

	limit, offset := defaultPagLimit, defaultPagOffset
	if l != "" {
		limit, err = strconv.Atoi(l)
		if err != nil || limit < 0 {
			return nil, kiterrors.InvalidArgument(fmt.Sprintf("invalid limit: %s", l))
		}
	}
	if o != "" {
		offset, err = strconv.Atoi(o)
		if err != nil || offset < 0 {
			return nil, kiterrors.InvalidArgument(fmt.Sprintf("invalid offset: %s", o))
		}
	}
	return Pagination(limit, offset), nil
}

func includedResourceObjectsFromURL(uri *url.URL) []Option {
	opts := []Option{}
	if len(uri.Query()) < 1 {
		return opts
	}
	if vals, exist := uri.Query()["include"]; exist && len(vals) > 0 {
		fNames := []string{}
		for _, val := range vals {
			for _, multiVal := range strings.Split(val, ",") { // multiple params splitted by coma for same value
				fNames = append(fNames, multiVal)
			}
		}
		opts = append(opts, IncludedResourceObjects(fNames...))
	}

	return opts
}

func sortingFromURL(uri *url.URL) []Option {
	q := uri.Query()
	raw := q.Get("sort")
	if raw == "" {
		return nil
	}
	pairs := []any{}
	for _, expr := range strings.Split(raw, ",") {
		expr = strings.TrimSpace(expr)
		if expr == "" {
			continue
		}
		dir := SortAsc
		switch expr[0] {
		case '-':
			dir = SortDesc
			expr = expr[1:]
		case '+':
			expr = expr[1:]
		}
		if expr == "" {
			continue
		}
		pairs = append(pairs, expr, dir)
	}
	if len(pairs) == 0 {
		return nil
	}
	return []Option{SortBy(pairs...)}
}

var aggOps = map[string]AggOp{
	"count": AggCount,
	"sum":   AggSum,
	"avg":   AggAvg,
	"min":   AggMin,
	"max":   AggMax,
}

func groupFromURL(uri *url.URL) []Option {
	raw := uri.Query().Get("group")
	if raw == "" {
		return nil
	}
	dims := []string{}
	for _, d := range strings.Split(raw, ",") {
		if d = strings.TrimSpace(d); d != "" {
			dims = append(dims, d)
		}
	}
	if len(dims) == 0 {
		return nil
	}
	return []Option{Group(dims...)}
}

func aggregationsFromURL(uri *url.URL) ([]Option, error) {
	opts := []Option{}
	for key, values := range uri.Query() {
		parts := strings.Split(key, "[")
		if len(parts) != 3 || parts[0] != "agg" {
			continue
		}
		alias := strings.TrimSuffix(parts[1], "]")
		opRaw := strings.TrimSuffix(parts[2], "]")
		if alias == "" || opRaw == "" || len(values) == 0 {
			continue
		}
		op, ok := aggOps[strings.ToLower(opRaw)]
		if !ok {
			return nil, kiterrors.InvalidArgument(fmt.Sprintf("invalid agg op %q (expected count/sum/avg/min/max)", opRaw))
		}
		field := strings.TrimSpace(values[0])
		opts = append(opts, Aggregate(alias, op, field))
	}
	return opts, nil
}

func bucketFromURL(uri *url.URL) ([]Option, error) {
	raw := uri.Query().Get("bucket")
	if raw == "" {
		return nil, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return nil, kiterrors.InvalidArgument(fmt.Sprintf("invalid bucket %q (expected a positive Go duration, e.g. 1h)", raw))
	}
	return []Option{Bucket(d)}, nil
}

func sparseFieldsetsFromURL(uri *url.URL) []Option {
	opts := []Option{}
	for key, values := range uri.Query() {
		if !strings.HasPrefix(key, "fields[") || !strings.HasSuffix(key, "]") {
			continue
		}
		resourceType := strings.TrimSuffix(strings.TrimPrefix(key, "fields["), "]")
		if resourceType == "" {
			continue
		}
		names := []string{}
		for _, v := range values {
			for _, n := range strings.Split(v, ",") {
				n = strings.TrimSpace(n)
				if n != "" {
					names = append(names, n)
				}
			}
		}
		if len(names) > 0 {
			opts = append(opts, Fields(resourceType, names...))
		}
	}
	return opts
}

type parseConfig struct {
	paginateByDefault bool
}

type ParseOpt func(c *parseConfig)

func DefaultPagination(applied bool) ParseOpt {
	return func(c *parseConfig) {
		c.paginateByDefault = applied
	}
}

func SkipDefaultPagination() ParseOpt {
	return DefaultPagination(false)
}

func defaultParseOpts() []ParseOpt {
	return []ParseOpt{
		DefaultPagination(true),
	}
}

func ParseURLQueryOpts(uri *url.URL, parseOpts ...ParseOpt) ([]Option, error) {
	config := new(parseConfig)
	pOpts := append(defaultParseOpts(), parseOpts...)
	for _, opt := range pOpts {
		opt(config)
	}

	opts, err := searchFromURL(uri)
	if err != nil {
		return nil, err
	}

	pag, err := paginationFromURL(uri, config.paginateByDefault)
	if err != nil {
		return nil, err
	}
	if pag != nil {
		opts = append(opts, pag)
	}

	if sortOpts := sortingFromURL(uri); len(sortOpts) > 0 {
		opts = append(opts, sortOpts...)
	}

	if includedResourceObjs := includedResourceObjectsFromURL(uri); len(includedResourceObjs) > 0 {
		opts = append(opts, includedResourceObjs...)
	}

	if fieldOpts := sparseFieldsetsFromURL(uri); len(fieldOpts) > 0 {
		opts = append(opts, fieldOpts...)
	}

	if groupOpts := groupFromURL(uri); len(groupOpts) > 0 {
		opts = append(opts, groupOpts...)
	}

	aggOpts, err := aggregationsFromURL(uri)
	if err != nil {
		return nil, err
	}
	opts = append(opts, aggOpts...)

	bucketOpts, err := bucketFromURL(uri)
	if err != nil {
		return nil, err
	}
	opts = append(opts, bucketOpts...)

	return opts, nil
}

func ParseOptsFromHTTPReq(r *http.Request, opts ...ParseOpt) ([]Option, error) {
	return ParseURLQueryOpts(r.URL, opts...)
}
