package searchtest

import (
	"github.com/fromforgesoftware/go-kit/filter"
	"github.com/fromforgesoftware/go-kit/search"
	"github.com/fromforgesoftware/go-kit/search/query"
)

func AnyOpts() []search.Option {
	return []search.Option{
		search.WithQueryOpts(query.FilterBy(filter.OpEq, "id", "id")),
	}
}
