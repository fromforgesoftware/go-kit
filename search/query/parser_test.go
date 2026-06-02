package query_test

import (
	"net/url"
	"testing"
	"time"

	"github.com/fromforgesoftware/go-kit/filter"
	"github.com/fromforgesoftware/go-kit/search/query"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseOperator_CanonicalNames(t *testing.T) {
	cases := map[string]filter.Operator{
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
	for wire, op := range cases {
		assert.Equal(t, op, query.ParseOperator(wire), "ParseOperator(%q)", wire)
	}
}

func TestParseOperator_LegacyAliasesRejected(t *testing.T) {
	assert.Equal(t, filter.OpUndefined, query.ParseOperator("is"))
	assert.Equal(t, filter.OpUndefined, query.ParseOperator("is-not"))
}

func TestParseOperator_Unknown(t *testing.T) {
	assert.Equal(t, filter.OpUndefined, query.ParseOperator("nope"))
}

func TestMarshalOperator_EmitsCanonicalNames(t *testing.T) {
	cases := map[filter.Operator]string{
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
	for op, expected := range cases {
		assert.Equal(t, expected, query.MarshalOperator(op), "MarshalOperator(%v)", op)
	}
}

func TestNewOpNotLike_RoundTrips(t *testing.T) {
	wire := query.MarshalOperator(filter.OpNotLike)
	assert.Equal(t, "not-like", wire)
	assert.Equal(t, filter.OpNotLike, query.ParseOperator(wire))
	assert.True(t, filter.OpNotLike.Valid())
	assert.Equal(t, "NOT LIKE", filter.OpNotLike.String())
}

func parseURL(t *testing.T, rawURL string) query.Query {
	t.Helper()
	u, err := url.Parse(rawURL)
	require.NoError(t, err)
	opts, err := query.ParseURLQueryOpts(u, query.SkipDefaultPagination())
	require.NoError(t, err)
	return query.New(opts...)
}

func TestParseURL_SortingExtracted(t *testing.T) {
	q := parseURL(t, "/articles?sort=name,-createdAt,+title")

	keys := q.Sorting().Keys()
	require.Equal(t, []string{"name", "createdAt", "title"}, keys)
	assert.Equal(t, query.SortAsc, q.Sorting().Get("name"))
	assert.Equal(t, query.SortDesc, q.Sorting().Get("createdAt"))
	assert.Equal(t, query.SortAsc, q.Sorting().Get("title"))
}

func TestParseURL_SortIgnoresEmptyAndPrefixOnly(t *testing.T) {
	q := parseURL(t, "/x?sort=,+,name")
	assert.Equal(t, []string{"name"}, q.Sorting().Keys())
}

func TestParseURL_SparseFieldsetsExtracted(t *testing.T) {
	q := parseURL(t, "/articles?fields[articles]=title,body&fields[authors]=name")

	require.Equal(t, []string{"title", "body"}, q.Fields()["articles"])
	require.Equal(t, []string{"name"}, q.Fields()["authors"])
}

func TestParseURL_SparseFieldsetsIgnoresEmpty(t *testing.T) {
	q := parseURL(t, "/x?fields[articles]=&fields[]=foo")
	assert.Empty(t, q.Fields())
}

func TestParseURL_PageNumberStyle(t *testing.T) {
	q := parseURL(t, "/x?page[number]=2&page[size]=20")
	require.NotNil(t, q.Pagination())
	assert.Equal(t, 20, q.Pagination().Limit)
	assert.Equal(t, 40, q.Pagination().Offset)
}

func TestParseURL_OffsetStyleStillWorks(t *testing.T) {
	q := parseURL(t, "/x?page[limit]=10&page[offset]=20")
	require.NotNil(t, q.Pagination())
	assert.Equal(t, 10, q.Pagination().Limit)
	assert.Equal(t, 20, q.Pagination().Offset)
}

func TestParseURL_StandalonePageSize(t *testing.T) {
	q := parseURL(t, "/x?page[size]=20")
	require.NotNil(t, q.Pagination())
	assert.Equal(t, 20, q.Pagination().Limit)
	assert.Equal(t, 0, q.Pagination().Offset)
}

func TestParseURL_StandalonePageNumberUsesDefaultSize(t *testing.T) {
	q := parseURL(t, "/x?page[number]=2")
	require.NotNil(t, q.Pagination())
	assert.Equal(t, 50, q.Pagination().Limit)
	assert.Equal(t, 100, q.Pagination().Offset)
}

func TestParseURL_DefaultPaginationLimit(t *testing.T) {
	u, err := url.Parse("/x")
	require.NoError(t, err)
	opts, err := query.ParseURLQueryOpts(u)
	require.NoError(t, err)
	q := query.New(opts...)
	require.NotNil(t, q.Pagination())
	assert.Equal(t, 50, q.Pagination().Limit)
	assert.Equal(t, 0, q.Pagination().Offset)
}

func TestParseURL_PageNumberTakesPrecedenceOverOffset(t *testing.T) {
	q := parseURL(t, "/x?page[number]=1&page[size]=5&page[limit]=999&page[offset]=999")
	require.NotNil(t, q.Pagination())
	assert.Equal(t, 5, q.Pagination().Limit)
	assert.Equal(t, 5, q.Pagination().Offset)
}

func TestParseURL_CursorStyleAfter(t *testing.T) {
	q := parseURL(t, "/x?page[after]=abc&page[size]=20")
	require.NotNil(t, q.Pagination())
	assert.True(t, q.Pagination().IsCursor())
	assert.Equal(t, "abc", q.Pagination().After)
	assert.Equal(t, "", q.Pagination().Before)
	assert.Equal(t, 20, q.Pagination().Size)
}

func TestParseURL_CursorStyleBefore(t *testing.T) {
	q := parseURL(t, "/x?page[before]=xyz&page[size]=10")
	require.NotNil(t, q.Pagination())
	assert.True(t, q.Pagination().IsCursor())
	assert.Equal(t, "xyz", q.Pagination().Before)
	assert.Equal(t, 10, q.Pagination().Size)
}

func TestParseURL_CursorWinsOverOffsetAndPageNumber(t *testing.T) {
	q := parseURL(t, "/x?page[after]=abc&page[size]=5&page[limit]=999&page[offset]=999&page[number]=42")
	require.NotNil(t, q.Pagination())
	assert.True(t, q.Pagination().IsCursor())
	assert.Equal(t, "abc", q.Pagination().After)
	assert.Equal(t, 5, q.Pagination().Size)
	assert.Equal(t, 0, q.Pagination().Limit)
	assert.Equal(t, 0, q.Pagination().Offset)
}

func TestParseURL_LegacyCursorParamIsIgnored(t *testing.T) {
	q := parseURL(t, "/x?cursor=legacy-base64&page[size]=15")
	if q.Pagination() != nil {
		assert.False(t, q.Pagination().IsCursor(), "legacy `cursor=` must not populate After")
		assert.Empty(t, q.Pagination().After)
	}
}

func TestParseURL_RejectsInvalidCursorSize(t *testing.T) {
	u, err := url.Parse("/x?page[after]=abc&page[size]=bad")
	require.NoError(t, err)
	_, err = query.ParseURLQueryOpts(u, query.SkipDefaultPagination())
	require.Error(t, err)
}

func TestPaginationToURLValues_OffsetStyle(t *testing.T) {
	v := query.PaginationToURLValues(&query.PaginationParams{Limit: 20, Offset: 40})
	assert.Equal(t, "20", v.Get("page[limit]"))
	assert.Equal(t, "40", v.Get("page[offset]"))
	assert.Empty(t, v.Get("page[after]"))
}

func TestPaginationToURLValues_CursorStyle(t *testing.T) {
	v := query.PaginationToURLValues(&query.PaginationParams{After: "abc", Size: 20})
	assert.Equal(t, "abc", v.Get("page[after]"))
	assert.Equal(t, "20", v.Get("page[size]"))
	assert.Empty(t, v.Get("page[limit]"))
	assert.Empty(t, v.Get("page[offset]"))
}

func TestPaginationToURLValues_Nil(t *testing.T) {
	assert.Empty(t, query.PaginationToURLValues(nil))
}

func TestEncodeDecodePositionCursor_RoundTrip(t *testing.T) {
	ts, err := time.Parse(time.RFC3339Nano, "2026-05-22T10:34:22.123456789Z")
	require.NoError(t, err)
	id := "00000000-0000-0000-0000-0000000000aa"

	encoded := query.EncodePositionCursor(ts, id)
	assert.NotEmpty(t, encoded)

	decTS, decID := query.DecodePositionCursor(encoded)
	assert.True(t, ts.Equal(decTS), "createdAt round-trip: got %v want %v", decTS, ts)
	assert.Equal(t, id, decID)
}

func TestEncodePositionCursor_ZeroInputsReturnEmpty(t *testing.T) {
	assert.Equal(t, "", query.EncodePositionCursor(time.Time{}, "id"))
	assert.Equal(t, "", query.EncodePositionCursor(time.Now(), ""))
}

func TestParseURL_GroupExtracted(t *testing.T) {
	q := parseURL(t, "/events?group=category,region,user.id")
	assert.Equal(t, []string{"category", "region", "user.id"}, q.Group())
}

func TestParseURL_GroupIgnoresEmpty(t *testing.T) {
	q := parseURL(t, "/x?group=,,category,")
	assert.Equal(t, []string{"category"}, q.Group())
}

func TestParseURL_AggregationsExtracted(t *testing.T) {
	q := parseURL(t, "/events?agg[totalAmount][sum]=amount&agg[count][count]=*&agg[avgLatency][avg]=latency")
	aggs := q.Aggregations()
	require.Len(t, aggs, 3)
	byAlias := map[string]query.Aggregation{}
	for _, a := range aggs {
		byAlias[a.Alias] = a
	}
	assert.Equal(t, query.AggSum, byAlias["totalAmount"].Operator)
	assert.Equal(t, "amount", byAlias["totalAmount"].Field)
	assert.Equal(t, query.AggCount, byAlias["count"].Operator)
	assert.Equal(t, "*", byAlias["count"].Field)
	assert.Equal(t, query.AggAvg, byAlias["avgLatency"].Operator)
}

func TestParseURL_AggregationsRejectsInvalidOp(t *testing.T) {
	u, err := url.Parse("/x?agg[x][median]=amount")
	require.NoError(t, err)
	_, err = query.ParseURLQueryOpts(u, query.SkipDefaultPagination())
	require.Error(t, err)
}

func TestParseURL_AggregationsIgnoresMalformedKey(t *testing.T) {
	q := parseURL(t, "/x?agg[x]=sumamount&agg[good][sum]=amount")
	aggs := q.Aggregations()
	require.Len(t, aggs, 1)
	assert.Equal(t, "good", aggs[0].Alias)
}

func TestParseURL_BucketExtracted(t *testing.T) {
	q := parseURL(t, "/events?bucket=1h")
	assert.Equal(t, time.Hour, q.Bucket())
}

func TestParseURL_BucketAcceptsMultipleUnits(t *testing.T) {
	for _, raw := range []string{"30s", "5m", "1h", "24h"} {
		q := parseURL(t, "/x?bucket="+raw)
		assert.Greater(t, q.Bucket(), time.Duration(0), "raw=%q", raw)
	}
}

func TestParseURL_BucketRejectsBadInput(t *testing.T) {
	for _, raw := range []string{"forever", "-1h", "0", "1yr"} {
		u, err := url.Parse("/x?bucket=" + raw)
		require.NoError(t, err)
		_, err = query.ParseURLQueryOpts(u, query.SkipDefaultPagination())
		require.Error(t, err, "raw=%q must be rejected", raw)
	}
}

func TestRoundTrip_GroupAggBucket(t *testing.T) {
	q := query.New(
		query.Group("category", "region"),
		query.Aggregate("totalAmount", query.AggSum, "amount"),
		query.Aggregate("count", query.AggCount, "*"),
		query.Bucket(time.Hour),
	)

	assert.Equal(t, "category,region", query.GroupToURLValue(q.Group()))

	aggs := query.AggregationsToURLValues(q.Aggregations())
	assert.Equal(t, "amount", aggs.Get("agg[totalAmount][sum]"))
	assert.Equal(t, "*", aggs.Get("agg[count][count]"))

	assert.Equal(t, "1h", query.BucketToURLValue(q.Bucket()))
}

func TestBucketToURLValue_PicksLargestUnit(t *testing.T) {
	assert.Equal(t, "1h", query.BucketToURLValue(time.Hour))
	assert.Equal(t, "30m", query.BucketToURLValue(30*time.Minute))
	assert.Equal(t, "168h", query.BucketToURLValue(7*24*time.Hour))
	assert.Equal(t, "", query.BucketToURLValue(0))
}

func TestParseURL_OrGroupsFlat(t *testing.T) {
	q := parseURL(t, "/tasks?filter[or][0][assignee][eq]=u-1&filter[or][1][createdBy][eq]=u-1")
	require.Len(t, q.OrGroups(), 2)
	assert.Equal(t, filter.OpEq, q.OrGroups()[0]["assignee"].Operator())
	assert.Equal(t, "u-1", q.OrGroups()[0]["assignee"].Value())
	assert.Equal(t, filter.OpEq, q.OrGroups()[1]["createdBy"].Operator())
}

func TestParseURL_OrGroupsDNF(t *testing.T) {
	q := parseURL(t, "/x?filter[or][0][role][eq]=admin&filter[or][0][tenant][eq]=A&filter[or][1][role][eq]=owner&filter[or][1][tenant][eq]=B")
	require.Len(t, q.OrGroups(), 2)
	assert.Equal(t, "admin", q.OrGroups()[0]["role"].Value())
	assert.Equal(t, "A", q.OrGroups()[0]["tenant"].Value())
	assert.Equal(t, "owner", q.OrGroups()[1]["role"].Value())
	assert.Equal(t, "B", q.OrGroups()[1]["tenant"].Value())
}

func TestParseURL_OrGroupsMixedWithTopLevel(t *testing.T) {
	q := parseURL(t, "/x?filter[status][eq]=active&filter[or][0][a][eq]=1&filter[or][1][b][eq]=2")
	require.NotNil(t, q.Filters().Get("status"))
	require.Len(t, q.OrGroups(), 2)
}

func TestParseURL_OrGroupsIndexesNotContiguous(t *testing.T) {
	q := parseURL(t, "/x?filter[or][0][a][eq]=1&filter[or][5][b][eq]=2")
	require.Len(t, q.OrGroups(), 2)
	assert.Equal(t, "1", q.OrGroups()[0]["a"].Value())
	assert.Equal(t, "2", q.OrGroups()[1]["b"].Value())
}

func TestParseURL_OrGroupRejectsBadIndex(t *testing.T) {
	u, err := url.Parse("/x?filter[or][abc][role][eq]=admin")
	require.NoError(t, err)
	_, err = query.ParseURLQueryOpts(u, query.SkipDefaultPagination())
	require.Error(t, err)
}

func TestParseURL_OrGroupRejectsInvalidOp(t *testing.T) {
	u, err := url.Parse("/x?filter[or][0][role][nope]=admin")
	require.NoError(t, err)
	_, err = query.ParseURLQueryOpts(u, query.SkipDefaultPagination())
	require.Error(t, err)
}

func TestOrGroupsToURLValues(t *testing.T) {
	q := query.New(
		query.OrGroup(
			query.FilterBy(filter.OpEq, "role", "admin"),
			query.FilterBy(filter.OpEq, "tenant", "A"),
		),
		query.OrGroup(
			query.FilterBy(filter.OpEq, "role", "owner"),
		),
	)
	v := query.OrGroupsToURLValues(q.OrGroups())
	assert.Equal(t, "admin", v.Get("filter[or][0][role][eq]"))
	assert.Equal(t, "A", v.Get("filter[or][0][tenant][eq]"))
	assert.Equal(t, "owner", v.Get("filter[or][1][role][eq]"))
}

func TestOrGroupOption_EmptyGroupIsDropped(t *testing.T) {
	q := query.New(query.OrGroup())
	assert.Empty(t, q.OrGroups())
}

func TestDecodePositionCursor_MalformedReturnsZero(t *testing.T) {
	ts, id := query.DecodePositionCursor("not-base64!!!")
	assert.True(t, ts.IsZero())
	assert.Equal(t, "", id)

	ts, id = query.DecodePositionCursor("")
	assert.True(t, ts.IsZero())
	assert.Equal(t, "", id)
}

func TestParseURL_FilterUsesNewWireNames(t *testing.T) {
	q := parseURL(t, "/users?filter[email][not-like]=%25@evil.com&filter[deletedAt][is-null]=true&filter[archivedAt][not-null]=true")

	emailF := q.Filters().Get("email")
	require.NotNil(t, emailF)
	assert.Equal(t, filter.OpNotLike, emailF.Operator())

	deletedF := q.Filters().Get("deletedAt")
	require.NotNil(t, deletedF)
	assert.Equal(t, filter.OpIsNull, deletedF.Operator())

	archivedF := q.Filters().Get("archivedAt")
	require.NotNil(t, archivedF)
	assert.Equal(t, filter.OpNotNull, archivedF.Operator())
}

func TestSortingToURLValue(t *testing.T) {
	q := query.New(
		query.SortBy("name", query.SortAsc, "createdAt", query.SortDesc, "title", query.SortAsc),
	)
	assert.Equal(t, "name,-createdAt,title", query.SortingToURLValue(q.Sorting()))
}

func TestSortingToURLValue_EmptyParams(t *testing.T) {
	q := query.New()
	assert.Equal(t, "", query.SortingToURLValue(q.Sorting()))
	assert.Equal(t, "", query.SortingToURLValue(nil))
}

func TestSparseFieldsetsToURLValues(t *testing.T) {
	q := query.New(
		query.Fields("articles", "title", "body"),
		query.Fields("authors", "name"),
	)
	values := query.SparseFieldsetsToURLValues(q.Fields())
	assert.Equal(t, "title,body", values.Get("fields[articles]"))
	assert.Equal(t, "name", values.Get("fields[authors]"))
}

func TestSparseFieldsetsToURLValues_EmptyMapEmits(t *testing.T) {
	values := query.SparseFieldsetsToURLValues(nil)
	assert.Empty(t, values)
}

func TestFiltersToURLValues_UsesNewWireNamesForNullChecks(t *testing.T) {
	q := query.New(
		query.FilterBy(filter.OpIsNull, "deletedAt", nil),
		query.FilterBy(filter.OpNotNull, "archivedAt", nil),
		query.FilterBy(filter.OpNotLike, "email", "%@evil.com"),
	)
	values := query.FiltersToURLValues(q.Filters())
	assert.Equal(t, "null", values.Get("filter[deletedAt][is-null]"))
	assert.Equal(t, "null", values.Get("filter[archivedAt][not-null]"))
	assert.Equal(t, "%@evil.com", values.Get("filter[email][not-like]"))
}

func TestRoundTrip_FilterPreservesNewOps(t *testing.T) {
	original, err := url.Parse("/u?filter[email][not-like]=%25@evil.com&filter[deletedAt][is-null]=null")
	require.NoError(t, err)
	opts, err := query.ParseURLQueryOpts(original, query.SkipDefaultPagination())
	require.NoError(t, err)
	q := query.New(opts...)

	rebuilt := query.FiltersToURLValues(q.Filters())
	assert.Equal(t, "%@evil.com", rebuilt.Get("filter[email][not-like]"))
	assert.Equal(t, "null", rebuilt.Get("filter[deletedAt][is-null]"))
}

func TestParseURL_LegacyWireNamesRejected(t *testing.T) {
	for _, raw := range []string{
		"/u?filter[deletedAt][is]=true",
		"/u?filter[archivedAt][is-not]=true",
	} {
		u, err := url.Parse(raw)
		require.NoError(t, err)
		_, err = query.ParseURLQueryOpts(u, query.SkipDefaultPagination())
		require.Error(t, err, "URL %q must be rejected: %s is no longer a valid operator", raw, raw)
	}
}
