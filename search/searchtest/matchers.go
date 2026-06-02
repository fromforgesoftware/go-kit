// Package searchtest provides test helpers for search options
package searchtest

import (
	"fmt"
	"reflect"

	"github.com/golang/mock/gomock"

	"github.com/fromforgesoftware/go-kit/search"
)

func OptsMatcherFunc(want ...search.Option) func([]search.Option) bool {
	return func(got []search.Option) bool {
		return reflect.DeepEqual(search.New(want...), search.New(got...))
	}
}

func OptMatcherFunc(want search.Option) func(search.Option) bool {
	return func(got search.Option) bool {
		return OptMatcher(want).Matches(got)
	}
}

func OptMatcher(opt search.Option) gomock.Matcher { return optMatcher{opt: opt} }

type optMatcher struct {
	opt search.Option
}

func (m optMatcher) Matches(x any) bool {
	q1 := search.New(m.opt)

	gotOpt, ok := x.(search.Option)
	if !ok {
		return false
	}
	q2 := search.New(gotOpt)
	return reflect.DeepEqual(q1, q2)
}

func (m optMatcher) String() string {
	return fmt.Sprintf("is equal to %v", search.New(m.opt))
}
