// Package querytest provides test helpers for query options
package querytest

import (
	"reflect"

	"github.com/fromforgesoftware/go-kit/search/query"
)

// OptMatcherFunc returns a matcher function that validates if the got query options
// produce the same Query state as the want options.
func OptMatcherFunc(want ...query.Option) func([]query.Option) bool {
	return func(got []query.Option) bool {
		q1 := query.New(want...)
		q2 := query.New(got...)
		return reflect.DeepEqual(q1, q2)
	}
}
