package postgres

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/filter"
	"github.com/fromforgesoftware/go-kit/search/query"
)

// These are white-box (package postgres) unit tests for the SQL filter builder.
// They cover the error path that previously panicked in filterOp's default
// case (repo.go), ensuring an unsupported operator now yields an error instead
// of crashing the service.

func TestFilterOp_SupportedOperators(t *testing.T) {
	cases := map[filter.Operator]string{
		filter.OpEq:           "=",
		filter.OpNEq:          "<>",
		filter.OpGT:           ">",
		filter.OpGTEq:         ">=",
		filter.OpLT:           "<",
		filter.OpLTEq:         "<=",
		filter.OpIn:           "IN",
		filter.OpNotIn:        "NOT IN",
		filter.OpLike:         "LIKE",
		filter.OpNotLike:      "NOT LIKE",
		filter.OpContainsLike: "LIKE ANY",
		filter.OpBetween:      "BETWEEN",
		filter.OpContains:     "@>",
		filter.OpIsNull:       "IS",
		filter.OpNotNull:      "IS NOT",
	}
	for op, want := range cases {
		got, err := filterOp(op)
		require.NoError(t, err, "op=%s", op)
		assert.Equal(t, want, got, "op=%s", op)
	}
}

func TestFilterOp_UnsupportedReturnsError(t *testing.T) {
	got, err := filterOp(filter.OpUndefined)
	require.Error(t, err)
	assert.Empty(t, got)
	assert.Contains(t, err.Error(), "is not supported")

	// An out-of-range operator value also returns an error rather than panicking.
	got, err = filterOp(filter.Operator(9999))
	require.Error(t, err)
	assert.Empty(t, got)
}

func TestSimpleArg_PropagatesUnsupportedOperatorError(t *testing.T) {
	_, err := simpleArg("status", filter.OpUndefined)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is not supported")
}

func TestSliceArg_PropagatesUnsupportedOperatorError(t *testing.T) {
	_, err := sliceArg(filter.OpUndefined, "status", []string{"a", "b"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is not supported")
}

func TestFilterClauseAnd_UnsupportedOperatorReturnsError(t *testing.T) {
	// filterClauseAnd only builds a SQL string from the field map; it needs no
	// live DB, so construct the Repo directly.
	repo := &Repo{fMapper: map[string]string{}}

	filters := query.Filters[any]{
		"status": filter.NewFieldFilter[any](filter.OpUndefined, "status", "active"),
	}
	_, _, ferr := repo.filterClauseAnd(filters, "events")
	require.Error(t, ferr)
	assert.Contains(t, ferr.Error(), "is not supported")
}

func TestFilterClauseAnd_SupportedOperatorBuildsClause(t *testing.T) {
	repo := &Repo{fMapper: map[string]string{"status": "status"}}
	filters := query.Filters[any]{
		"status": filter.NewFieldFilter[any](filter.OpEq, "status", "active"),
	}
	clause, args, err := repo.filterClauseAnd(filters, "events")
	require.NoError(t, err)
	assert.Equal(t, "events.status = ?", clause)
	require.Len(t, args, 1)
	assert.Equal(t, "active", args[0])
}
