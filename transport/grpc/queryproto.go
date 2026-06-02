package grpc

import (
	"github.com/fromforgesoftware/go-kit/filter"
	searchpb "github.com/fromforgesoftware/go-kit/proto/tb/search/v1"
	"github.com/fromforgesoftware/go-kit/search"
	"github.com/fromforgesoftware/go-kit/search/query"
)

// QueryOptsFromProto translates a searchpb.QueryOptions message into
// the same []search.Option a REST handler would build from
// ?filter[field][op]=value&page[limit]=…&sort=…&include=…. Lets gRPC
// and REST share one filter contract.
func QueryOptsFromProto(qo *searchpb.QueryOptions) []search.Option {
	if qo == nil {
		return nil
	}

	queryOpts := make([]query.Option, 0, len(qo.GetFilters())+3)

	for _, f := range qo.GetFilters() {
		opt := filterOptionFromProto(f)
		if opt == nil {
			continue
		}
		queryOpts = append(queryOpts, opt)
	}

	if p := qo.GetPagination(); p != nil {
		if p.GetBefore() != "" || p.GetAfter() != "" {
			queryOpts = append(queryOpts, query.CursorPagination(p.GetBefore(), p.GetAfter(), int(p.GetSize())))
		} else if p.GetLimit() > 0 || p.GetOffset() > 0 {
			queryOpts = append(queryOpts, query.Pagination(int(p.GetLimit()), int(p.GetOffset())))
		}
	}

	if fs := qo.GetFields(); len(fs) > 0 {
		for _, fset := range fs {
			if fset.GetResourceType() == "" || len(fset.GetNames()) == 0 {
				continue
			}
			queryOpts = append(queryOpts, query.Fields(fset.GetResourceType(), fset.GetNames()...))
		}
	}

	if sorts := qo.GetSort(); len(sorts) > 0 {
		params := make([]any, 0, len(sorts)*2)
		for _, s := range sorts {
			if s.GetField() == "" {
				continue
			}
			dir := query.SortAsc
			if s.GetDescending() {
				dir = query.SortDesc
			}
			params = append(params, s.GetField(), dir)
		}
		if len(params) > 0 {
			queryOpts = append(queryOpts, query.SortBy(params...))
		}
	}

	if includes := qo.GetInclude(); len(includes) > 0 {
		queryOpts = append(queryOpts, query.IncludedResourceObjects(includes...))
	}

	return []search.Option{search.WithQueryOpts(queryOpts...)}
}

// filterOptionFromProto maps a single proto Filter to a query.Option.
// Returns nil for malformed filters (no field, no values, or
// unspecified operator).
func filterOptionFromProto(f *searchpb.Filter) query.Option {
	if f == nil || f.GetField() == "" {
		return nil
	}
	op := operatorFromProto(f.GetOperator())
	if !op.Valid() {
		return nil
	}
	vals := f.GetValues()
	if len(vals) == 0 {
		return nil
	}
	// Multi-value operators (IN / NOT IN / BETWEEN / CONTAINS_LIKE)
	// take the full slice; single-value operators read the first.
	switch op {
	case filter.OpIn, filter.OpNotIn, filter.OpBetween, filter.OpContainsLike:
		return query.FilterBy(op, f.GetField(), vals)
	default:
		return query.FilterBy(op, f.GetField(), vals[0])
	}
}

// operatorFromProto is the explicit ordinal mapping. The proto enum
// matches filter.Operator 1:1 today, but the explicit switch keeps
// the wire contract stable if either side reorders.
func operatorFromProto(op searchpb.Operator) filter.Operator {
	switch op {
	case searchpb.Operator_OPERATOR_EQ:
		return filter.OpEq
	case searchpb.Operator_OPERATOR_NEQ:
		return filter.OpNEq
	case searchpb.Operator_OPERATOR_GT:
		return filter.OpGT
	case searchpb.Operator_OPERATOR_GTEQ:
		return filter.OpGTEq
	case searchpb.Operator_OPERATOR_LT:
		return filter.OpLT
	case searchpb.Operator_OPERATOR_LTEQ:
		return filter.OpLTEq
	case searchpb.Operator_OPERATOR_IN:
		return filter.OpIn
	case searchpb.Operator_OPERATOR_NOT_IN:
		return filter.OpNotIn
	case searchpb.Operator_OPERATOR_LIKE:
		return filter.OpLike
	case searchpb.Operator_OPERATOR_BETWEEN:
		return filter.OpBetween
	case searchpb.Operator_OPERATOR_CONTAINS:
		return filter.OpContains
	case searchpb.Operator_OPERATOR_CONTAINS_LIKE:
		return filter.OpContainsLike
	case searchpb.Operator_OPERATOR_IS_NULL:
		return filter.OpIsNull
	case searchpb.Operator_OPERATOR_NOT_NULL:
		return filter.OpNotNull
	case searchpb.Operator_OPERATOR_NOT_LIKE:
		return filter.OpNotLike
	}
	return filter.OpUndefined
}
