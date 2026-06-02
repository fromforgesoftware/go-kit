package searchtest_test

import (
	"testing"

	"github.com/fromforgesoftware/go-kit/search"
	"github.com/fromforgesoftware/go-kit/search/query"
	"github.com/fromforgesoftware/go-kit/search/searchtest"
)

func TestOptsHelpers(t *testing.T) {
	tests := []struct {
		name      string
		one       []search.Option
		another   []search.Option
		mustEqual bool
	}{
		{
			name: "both empty", mustEqual: true,
			one: []search.Option{}, another: []search.Option{},
		},
		{
			name: "both nil", mustEqual: true,
			one: nil, another: nil,
		},
		{
			name: "one with query, another without", mustEqual: false,
			one: []search.Option{}, another: []search.Option{search.WithQuery(query.New())},
		},
		{
			name: "single opt, different args", mustEqual: false,
			one: []search.Option{
				search.WithQueryOpts(
					query.SortBy("createdAt", query.SortDesc),
				),
			},
			another: []search.Option{
				search.WithQueryOpts(
					query.SortBy("createdAt", query.SortAsc),
				),
			},
		},
		{
			name: "single opt, same args", mustEqual: true,
			one: []search.Option{
				search.WithQueryOpts(
					query.SortBy("createdAt", query.SortDesc),
				),
			},
			another: []search.Option{
				search.WithQueryOpts(
					query.SortBy("createdAt", query.SortDesc),
				),
			},
		},
		{
			name: "different opt count, first same args", mustEqual: false,
			one: []search.Option{
				search.WithQueryOpts(
					query.SortBy("createdAt", query.SortDesc),
				),
			},
			another: []search.Option{
				search.WithQueryOpts(
					query.SortBy("createdAt", query.SortDesc),
				),
				search.WithQueryOpts(
					query.SortBy("updatedAt", query.SortAsc),
				),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.mustEqual {
				searchtest.OptsEqual(t, test.one, test.another)
			} else {
				searchtest.OptsDiff(t, test.one, test.another)
			}
		})
	}
}
