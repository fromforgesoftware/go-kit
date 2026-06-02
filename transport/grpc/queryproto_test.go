package grpc_test

import (
	"testing"

	"github.com/fromforgesoftware/go-kit/filter"
	searchpb "github.com/fromforgesoftware/go-kit/proto/tb/search/v1"
	"github.com/fromforgesoftware/go-kit/search"
	kitgrpc "github.com/fromforgesoftware/go-kit/transport/grpc"
	"github.com/stretchr/testify/assert"
)

func TestQueryOptsFromProto_NilAndEmpty(t *testing.T) {
	assert.Nil(t, kitgrpc.QueryOptsFromProto(nil))
	// Empty QueryOptions still returns a wrapped option set (no
	// filters/pagination/etc.), which downstream code treats as a
	// no-op query.
	got := kitgrpc.QueryOptsFromProto(&searchpb.QueryOptions{})
	assert.Len(t, got, 1)
}

func TestQueryOptsFromProto_SingleValueFilter(t *testing.T) {
	qo := &searchpb.QueryOptions{
		Filters: []*searchpb.Filter{
			{Field: "name", Operator: searchpb.Operator_OPERATOR_EQ, Values: []string{"acme"}},
		},
	}

	got := search.New(kitgrpc.QueryOptsFromProto(qo)...)
	f := got.Query().Filters().Get("name")
	if assert.NotNil(t, f) {
		assert.Equal(t, filter.OpEq, f.Operator())
		assert.Equal(t, "acme", f.Value())
	}
}

func TestQueryOptsFromProto_InFilterUsesSlice(t *testing.T) {
	qo := &searchpb.QueryOptions{
		Filters: []*searchpb.Filter{
			{Field: "status", Operator: searchpb.Operator_OPERATOR_IN, Values: []string{"a", "b", "c"}},
		},
	}

	got := search.New(kitgrpc.QueryOptsFromProto(qo)...)
	f := got.Query().Filters().Get("status")
	if assert.NotNil(t, f) {
		assert.Equal(t, filter.OpIn, f.Operator())
		assert.Equal(t, []string{"a", "b", "c"}, f.Value())
	}
}

func TestQueryOptsFromProto_DropsMalformed(t *testing.T) {
	qo := &searchpb.QueryOptions{
		Filters: []*searchpb.Filter{
			{Field: "", Operator: searchpb.Operator_OPERATOR_EQ, Values: []string{"x"}}, // no field
			{Field: "x", Operator: searchpb.Operator_OPERATOR_UNSPECIFIED, Values: []string{"x"}},
			{Field: "y", Operator: searchpb.Operator_OPERATOR_EQ, Values: nil}, // no values
		},
	}

	got := search.New(kitgrpc.QueryOptsFromProto(qo)...)
	for _, k := range []string{"", "x", "y"} {
		assert.Nil(t, got.Query().Filters().Get(k), "field %q should not have been registered", k)
	}
}

func TestQueryOptsFromProto_Pagination(t *testing.T) {
	qo := &searchpb.QueryOptions{
		Pagination: &searchpb.Pagination{Limit: 50, Offset: 100},
	}

	got := search.New(kitgrpc.QueryOptsFromProto(qo)...)
	p := got.Query().Pagination()
	if assert.NotNil(t, p) {
		assert.Equal(t, 50, p.Limit)
		assert.Equal(t, 100, p.Offset)
	}
}

func TestQueryOptsFromProto_Sort(t *testing.T) {
	qo := &searchpb.QueryOptions{
		Sort: []*searchpb.Sort{
			{Field: "created_at", Descending: true},
			{Field: "name", Descending: false},
		},
	}

	got := search.New(kitgrpc.QueryOptsFromProto(qo)...)
	keys := got.Query().Sorting().Keys()
	assert.ElementsMatch(t, []string{"created_at", "name"}, keys)
}

func TestQueryOptsFromProto_Include(t *testing.T) {
	qo := &searchpb.QueryOptions{Include: []string{"members", "owner"}}

	got := search.New(kitgrpc.QueryOptsFromProto(qo)...)
	assert.ElementsMatch(t, []string{"members", "owner"}, got.Query().IncludedResourceObjects())
}

func TestQueryOptsFromProto_NotLikeOperator(t *testing.T) {
	qo := &searchpb.QueryOptions{
		Filters: []*searchpb.Filter{
			{Field: "email", Operator: searchpb.Operator_OPERATOR_NOT_LIKE, Values: []string{"%@evil.com"}},
		},
	}
	got := search.New(kitgrpc.QueryOptsFromProto(qo)...)
	f := got.Query().Filters().Get("email")
	if assert.NotNil(t, f) {
		assert.Equal(t, filter.OpNotLike, f.Operator())
		assert.Equal(t, "%@evil.com", f.Value())
	}
}

func TestQueryOptsFromProto_CursorPagination(t *testing.T) {
	qo := &searchpb.QueryOptions{
		Pagination: &searchpb.Pagination{After: "abc", Size: 20},
	}
	got := search.New(kitgrpc.QueryOptsFromProto(qo)...)
	p := got.Query().Pagination()
	if assert.NotNil(t, p) {
		assert.True(t, p.IsCursor())
		assert.Equal(t, "abc", p.After)
		assert.Equal(t, 20, p.Size)
	}
}

func TestQueryOptsFromProto_CursorWinsOverOffset(t *testing.T) {
	qo := &searchpb.QueryOptions{
		Pagination: &searchpb.Pagination{Before: "xyz", Size: 10, Limit: 999, Offset: 999},
	}
	got := search.New(kitgrpc.QueryOptsFromProto(qo)...)
	p := got.Query().Pagination()
	if assert.NotNil(t, p) {
		assert.True(t, p.IsCursor())
		assert.Equal(t, "xyz", p.Before)
		assert.Equal(t, 10, p.Size)
		assert.Equal(t, 0, p.Limit)
		assert.Equal(t, 0, p.Offset)
	}
}

func TestQueryOptsFromProto_SparseFieldsets(t *testing.T) {
	qo := &searchpb.QueryOptions{
		Fields: []*searchpb.FieldSet{
			{ResourceType: "articles", Names: []string{"title", "body"}},
			{ResourceType: "authors", Names: []string{"name"}},
		},
	}
	got := search.New(kitgrpc.QueryOptsFromProto(qo)...)
	assert.Equal(t, []string{"title", "body"}, got.Query().Fields()["articles"])
	assert.Equal(t, []string{"name"}, got.Query().Fields()["authors"])
}
