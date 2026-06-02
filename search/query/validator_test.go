package query_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/filter"
	"github.com/fromforgesoftware/go-kit/search/query"
)

func TestValidator_MandatoryFilterMissing(t *testing.T) {
	q := query.New(query.FilterBy(filter.OpEq, "status", "x"))
	err := query.Validate(q,
		query.MandatoryFilters("workspaceId"),
		query.OptionalFilters("status"),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "workspaceId")
}

func TestValidator_MandatoryFilterPresent(t *testing.T) {
	q := query.New(query.FilterBy(filter.OpEq, "workspaceId", "ws-1"))
	err := query.Validate(q, query.MandatoryFilters("workspaceId"))
	require.NoError(t, err)
}

func TestValidator_DisallowedFilterFieldRejected(t *testing.T) {
	q := query.New(query.FilterBy(filter.OpEq, "secret", "x"))
	err := query.Validate(q, query.OptionalFilters("status", "name"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "secret")
}

func TestValidator_AtLeastOneFilterRequired(t *testing.T) {
	require.Error(t, query.Validate(query.New(), query.AtLeastOneFilter(), query.OptionalFilters("x")))
	require.NoError(t, query.Validate(
		query.New(query.FilterBy(filter.OpEq, "x", 1)),
		query.AtLeastOneFilter(),
		query.OptionalFilters("x"),
	))
}

func TestValidator_GroupedFiltersBothOrNone(t *testing.T) {
	opts := []query.ValidationOpt{
		query.GroupedFilters("startDate", "endDate"),
		query.OptionalFilters("startDate", "endDate"),
	}
	require.NoError(t, query.Validate(query.New(), opts...))
	require.NoError(t, query.Validate(
		query.New(
			query.FilterBy(filter.OpGTEq, "startDate", "2026-01-01"),
			query.FilterBy(filter.OpLTEq, "endDate", "2026-12-31"),
		),
		opts...,
	))
	err := query.Validate(
		query.New(query.FilterBy(filter.OpGTEq, "startDate", "2026-01-01")),
		opts...,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "together")
}

func TestValidator_SortFieldAllowlist(t *testing.T) {
	q := query.New(query.SortBy("disallowed", query.SortAsc))
	err := query.Validate(q, query.SortFields("createdAt"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "disallowed")
}

func TestValidator_FilterValueFunc(t *testing.T) {
	notEmpty := func(f filter.FieldFilter[any]) error {
		if s, _ := f.Value().(string); s == "" {
			return assert.AnError
		}
		return nil
	}
	require.NoError(t, query.Validate(
		query.New(query.FilterBy(filter.OpEq, "name", "alice")),
		query.OptionalFilters("name"),
		query.ValidFilter("name", notEmpty),
	))
	require.Error(t, query.Validate(
		query.New(query.FilterBy(filter.OpEq, "name", "")),
		query.OptionalFilters("name"),
		query.ValidFilter("name", notEmpty),
	))
}

func TestValidator_GroupAllowed(t *testing.T) {
	q := query.New(query.Group("category", "region"))
	err := query.Validate(q,
		query.OptionalFilters(),
		query.GroupFields("category", "region", "user.id"),
	)
	require.NoError(t, err)
}

func TestValidator_GroupDisallowed(t *testing.T) {
	q := query.New(query.Group("category", "ssn"))
	err := query.Validate(q,
		query.OptionalFilters(),
		query.GroupFields("category", "region"),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ssn")
}

func TestValidator_GroupNoneIsOK(t *testing.T) {
	require.NoError(t, query.Validate(query.New(), query.GroupFields("category")))
}

func TestValidator_GroupAnyDisallowedByDefault(t *testing.T) {
	q := query.New(query.Group("category"))
	err := query.Validate(q)
	require.Error(t, err, "no allowlist → all group fields rejected")
}

func TestValidator_AggregateAllowedField(t *testing.T) {
	q := query.New(
		query.Aggregate("totalAmount", query.AggSum, "amount"),
		query.Aggregate("count", query.AggCount, "*"),
	)
	err := query.Validate(q,
		query.OptionalFilters(),
		query.AggregationFields("amount"),
	)
	require.NoError(t, err)
}

func TestValidator_AggregateDisallowedField(t *testing.T) {
	q := query.New(query.Aggregate("total", query.AggSum, "ssn"))
	err := query.Validate(q,
		query.OptionalFilters(),
		query.AggregationFields("amount", "latency"),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ssn")
}

func TestValidator_AggregateStarAlwaysAllowed(t *testing.T) {
	q := query.New(query.Aggregate("count", query.AggCount, "*"))
	require.NoError(t, query.Validate(q))
}

func TestValidator_AggregateDuplicateAliasRejected(t *testing.T) {
	q := query.New(
		query.Aggregate("total", query.AggSum, "amount"),
		query.Aggregate("total", query.AggCount, "*"),
	)
	err := query.Validate(q, query.AggregationFields("amount"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
}

func TestValidator_AggregateNoneIsOK(t *testing.T) {
	require.NoError(t, query.Validate(query.New()))
}

func TestValidator_OrGroupFieldsGoThroughSameAllowlist(t *testing.T) {
	q := query.New(
		query.OrGroup(query.FilterBy(filter.OpEq, "role", "admin")),
		query.OrGroup(query.FilterBy(filter.OpEq, "tenant", "A")),
	)
	require.NoError(t, query.Validate(q, query.OptionalFilters("role", "tenant")))
}

func TestValidator_OrGroupFieldRejectedIfNotAllowed(t *testing.T) {
	q := query.New(query.OrGroup(query.FilterBy(filter.OpEq, "ssn", "x")))
	err := query.Validate(q, query.OptionalFilters("role"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ssn")
}

func TestValidator_OrGroupMandatoryFieldNotCountedAsTopLevel(t *testing.T) {
	q := query.New(query.OrGroup(query.FilterBy(filter.OpEq, "workspaceId", "ws-1")))
	err := query.Validate(q, query.MandatoryFilters("workspaceId"))
	require.Error(t, err, "OR-group filters don't satisfy top-level mandatory")
}
